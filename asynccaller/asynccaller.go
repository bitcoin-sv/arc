package asynccaller

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/labstack/gommon/random"
	"github.com/ordishs/go-utils"
	"github.com/ordishs/go-utils/batcher"
)

const ISO8601 = "2006-01-02T15:04:05.999Z"

type AsyncCaller[T any] struct {
	logger       utils.Logger
	dirName      string
	ch           chan *T
	callerClient CallerClientI[T]
}

func New[T any](logger utils.Logger, dirName string, delay time.Duration, callerClient CallerClientI[T]) (*AsyncCaller[T], error) {

	if err := os.MkdirAll(dirName, os.ModePerm); err != nil {
		return nil, fmt.Errorf("could not create directory %s: %v", dirName, err)
	}

	a := &AsyncCaller[T]{
		logger:       logger,
		dirName:      dirName,
		ch:           make(chan *T),
		callerClient: callerClient,
	}

	a.init(delay)

	return a, nil
}

func (a *AsyncCaller[T]) init(delay time.Duration) {
	batch := a.getBatcher(delay)
	go func() {
		for {
			a.processFiles()
			time.Sleep(10 * time.Second)
		}
	}()

	go func() {
		for data := range a.ch {
			a.logger.Infof("calling caller client: %v", data)
			if err := a.callerClient.Caller(data); err != nil {
				a.logger.Errorf("error calling caller: %v", err)
				batch.Put(data)
			}
		}
	}()
}

func (a *AsyncCaller[T]) Call(data *T) {
	a.ch <- data
}

func (a *AsyncCaller[T]) GetChannel() chan *T {
	return a.ch
}

func (a *AsyncCaller[T]) getBatcher(delay time.Duration) *batcher.Batcher[T] {
	return batcher.New[T](10000, delay, func(batch []*T) {
		// write the batch to file
		a.logger.Infof("writing batch of %d transactions to file", len(batch))

		fileName := fmt.Sprintf("%s/batch-%s-%s.csv", a.dirName, time.Now().Format(ISO8601), random.String(4))
		f, err := os.Create(fileName)
		if err != nil {
			a.logger.Errorf("could not create file %s: %v", fileName, err)
			return
		}
		defer f.Close()

		var line string
		for _, item := range batch {
			if line, err = a.callerClient.MarshalString(item); err != nil {
				a.logger.Errorf("could not serialize batcher item to string: %v", err)
				return
			}
			_, err = f.WriteString(line + "\n")
			if err != nil {
				a.logger.Errorf("could not write to file %s: %v", fileName, err)
				return
			}
		}
	}, true)
}

func (a *AsyncCaller[T]) processFiles() {
	// check whether there is a batch file to process
	files, err := os.ReadDir(a.dirName)
	if err != nil {
		a.logger.Errorf("error reading directory: %v", err)
		return
	}

	for _, file := range files {
		a.logger.Infof("processing file %s", file.Name())
		a.logger.Infof("processing file %s/%s", a.dirName, file.Name())
		if !strings.HasPrefix(file.Name(), "batch-") || !strings.HasSuffix(file.Name(), ".csv") {
			continue
		}

		fullName := path.Join(a.dirName, file.Name())
		a.logger.Infof("found batch file %s, processing", fullName)

		if err = a.processFile(fullName); err != nil {
			a.logger.Errorf("error processing file %s: %v", fullName, err)
			continue
		}

		// delete the file
		if err = os.Remove(fullName); err != nil {
			a.logger.Errorf("error deleting file %s: %v", fullName, err)
		}
	}
}

func (a *AsyncCaller[T]) processFile(fileName string) error {
	f, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("could not open file %s: %v", fileName, err)
	}
	defer f.Close()

	// read the file line by line
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		var data *T
		if data, err = a.callerClient.UnmarshalString(line); err != nil {
			return fmt.Errorf("could not deserialize string to batcher item: %v", err)
		} else {
			if err = a.callerClient.Caller(data); err != nil {
				return fmt.Errorf("error calling caller: %v", err)
			}
		}
	}

	return nil
}
