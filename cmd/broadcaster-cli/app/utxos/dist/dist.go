package dist

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/woc_client"
	"github.com/cenkalti/backoff/v4"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/libsv/go-bt/v2"
	"github.com/lmittmann/tint"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:   "dist",
	Short: "Show distribution of utxo sizes in key set",
	RunE: func(cmd *cobra.Command, args []string) error {
		maxRows := viper.GetInt("rows")

		keyFile, err := helper.GetString("keyFile")
		if err != nil {
			return err
		}
		if keyFile == "" {
			return errors.New("no key file was given")
		}

		isTestnet, err := helper.GetBool("testnet")
		if err != nil {
			return err
		}
		wocApiKey, err := helper.GetString("wocAPIKey")
		if err != nil {
			return err
		}
		wocClient := woc_client.New(woc_client.WithAuth(wocApiKey))

		keyFiles := strings.Split(keyFile, ",")
		t := table.NewWriter()

		type row struct {
			satoshis string
			outputs  string
		}
		logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelInfo}))
		columns := make([][]row, len(keyFiles))
		maxRowNr := 0

		keyTotalOutputs := make([]int, len(keyFiles))
		keyHeaderRow := make([]interface{}, 0)
		headerRow := make([]interface{}, 0)
		for i, kf := range keyFiles {

			_, keyName := filepath.Split(kf)
			headerRow = append(headerRow, "Sat", "Outputs")
			keyHeaderRow = append(keyHeaderRow, keyName, "")

			fundingKeySet, _, err := helper.GetKeySetsKeyFile(kf)
			if err != nil {
				return fmt.Errorf("failed to get key sets: %v", err)
			}

			if err != nil {
				return err
			}

			policy := backoff.WithMaxRetries(backoff.NewConstantBackOff(1*time.Second), 5)

			operation := func() ([]*bt.UTXO, error) {
				wocUtxos, err := wocClient.GetUTXOs(!isTestnet, fundingKeySet.Script, fundingKeySet.Address(!isTestnet))
				if err != nil {
					return nil, fmt.Errorf("failed to get utxos from WoC: %v", err)
				}
				return wocUtxos, nil
			}

			notify := func(err error, nextTry time.Duration) {
				logger.Error("failed to get utxos from WoC", slog.String("key", keyName), slog.String("address", fundingKeySet.Address(!isTestnet)), slog.String("next try", nextTry.String()), slog.String("err", err.Error()))
			}

			utxos, err := backoff.RetryNotifyWithData(operation, policy, notify)
			if err != nil {
				continue
			}

			outputsMap := map[uint64]int{}
			satoshiSlice := []uint64{}
			var found bool
			for _, utxo := range utxos {
				_, found = outputsMap[utxo.Satoshis]
				if found {
					outputsMap[utxo.Satoshis]++
					continue
				}

				outputsMap[utxo.Satoshis] = 1

				satoshiSlice = append(satoshiSlice, utxo.Satoshis)
			}

			sort.Slice(satoshiSlice, func(i, j int) bool {
				return satoshiSlice[j] < satoshiSlice[i]
			})

			totalOutputs := 0

			for _, satoshi := range satoshiSlice {

				columns[i] = append(columns[i], row{
					satoshis: strconv.FormatUint(satoshi, 10),
					outputs:  strconv.Itoa(outputsMap[satoshi]),
				})

				totalOutputs += outputsMap[satoshi]
			}

			keyTotalOutputs[i] = totalOutputs

			if len(columns[i]) > maxRowNr {
				maxRowNr = len(columns[i])
			}
		}

		t.AppendHeader(keyHeaderRow)
		t.AppendHeader(headerRow)

		rows := make([][]string, maxRowNr)

		for i := 0; i < maxRowNr; i++ {
			for j := range columns {

				if len(columns[j]) < i+1 {
					rows[i] = append(rows[i], "")
					rows[i] = append(rows[i], "")
					continue
				}

				rows[i] = append(rows[i], columns[j][i].satoshis)
				rows[i] = append(rows[i], columns[j][i].outputs)
			}
		}

		for i, row := range rows {
			tableRow := table.Row{}

			if maxRows != 0 && i == maxRows+1 {
				for range row {
					tableRow = append(tableRow, "...")
				}

				t.AppendRow(tableRow)

				continue
			}

			if maxRows != 0 && i > maxRows {
				continue
			}

			for _, rowVal := range row {
				tableRow = append(tableRow, rowVal)
			}

			t.AppendRow(tableRow)
		}

		totalRow := table.Row{}
		for _, total := range keyTotalOutputs {
			totalRow = append(totalRow, "Total", total)
		}
		t.AppendRow(totalRow)

		fmt.Println(t.Render())

		return nil
	},
}

func init() {
	var err error

	Cmd.Flags().IntP("rows", "r", 0, "Maximum rows to show - default: all")
	err = viper.BindPFlag("rows", Cmd.Flags().Lookup("rows"))
	if err != nil {
		log.Fatal(err)
	}
}
