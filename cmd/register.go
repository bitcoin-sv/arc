package cmd

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/TAAL-GmbH/arc/blocktx"
	"github.com/TAAL-GmbH/arc/blocktx/blocktx_api"
	"github.com/labstack/gommon/random"
	"github.com/libsv/go-bt/v2"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/go-utils/batcher"
	"github.com/ordishs/gocore"
)

func initRegisterTransactionChannel(logger *gocore.Logger, btc blocktx.ClientI) chan *blocktx_api.TransactionAndSource {
	dirName := "./tx-register"
	if err := os.MkdirAll(dirName, os.ModePerm); err != nil {
		logger.Fatalf("could not create directory %s: %v", dirName, err)
	}
	batch := getRegisterTransactionBatcher(logger, dirName)
	go processRegisterTransactionFiles(logger, btc, dirName)

	registerCh := make(chan *blocktx_api.TransactionAndSource)
	go func() {
		for tx := range registerCh {
			if err := btc.RegisterTransaction(context.Background(), tx); err != nil {
				logger.Errorf("error registering transaction %x: %v", bt.ReverseBytes(tx.Hash), err)
				batch.Put(tx)
			}
		}
	}()

	return registerCh
}

func getRegisterTransactionBatcher(logger *gocore.Logger, dirName string) *batcher.Batcher[blocktx_api.TransactionAndSource] {
	return batcher.New[blocktx_api.TransactionAndSource](10000, 10*time.Second, func(batch []*blocktx_api.TransactionAndSource) {
		// write the batch to file
		logger.Infof("writing batch of %d transactions to file", len(batch))

		fileName := fmt.Sprintf("%s/batch-%s-%s.csv", dirName, time.Now().Format(ISO8601), random.String(4))
		f, err := os.Create(fileName)
		if err != nil {
			logger.Errorf("could not create file %s: %v", fileName, err)
			return
		}
		defer f.Close()

		for _, tx := range batch {
			_, err = f.WriteString(fmt.Sprintf("%x,%s\n", bt.ReverseBytes((*tx).Hash), (*tx).Source))
			if err != nil {
				logger.Errorf("could not write to file %s: %v", fileName, err)
				return
			}
		}
	}, true)
}

func processRegisterTransactionFiles(logger utils.Logger, btc blocktx.ClientI, dirName string) {
	for {
		// check whether there is a batch file to process
		files, err := os.ReadDir(dirName)
		if err != nil {
			logger.Errorf("error reading directory: %v", err)
			continue
		}

		for _, file := range files {
			logger.Infof("processing file %s", file.Name())
			logger.Infof("processing file %s/%s", dirName, file.Name())
			if !strings.HasPrefix(file.Name(), "batch-") || !strings.HasSuffix(file.Name(), ".csv") {
				continue
			}

			fullName := path.Join(dirName, file.Name())
			logger.Infof("found batch file %s, processing", fullName)

			if err = processFile(btc, fullName); err != nil {
				logger.Errorf("error processing file %s: %v", fullName, err)
				continue
			}

			// delete the file
			if err = os.Remove(fullName); err != nil {
				logger.Errorf("error deleting file %s: %v", fullName, err)
			}
		}

		time.Sleep(10 * time.Second)
	}
}

func processFile(btc blocktx.ClientI, fileName string) error {
	f, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("could not open file %s: %v", fileName, err)
	}
	defer f.Close()

	// read the file line by line
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			return fmt.Errorf("invalid line in file %s: %s", fileName, line)
		}

		var hash []byte
		hash, err = hex.DecodeString(parts[0])
		if err != nil {
			return fmt.Errorf("invalid hash in file %s: %s", fileName, line)
		}

		if err = btc.RegisterTransaction(context.Background(), &blocktx_api.TransactionAndSource{
			Hash:   bt.ReverseBytes(hash),
			Source: parts[1],
		}); err != nil {
			return fmt.Errorf("error registering transaction %x: %v", bt.ReverseBytes(hash), err)
		}
	}

	return nil
}
