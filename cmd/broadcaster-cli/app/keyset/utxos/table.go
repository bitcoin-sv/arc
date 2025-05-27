package utxos

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"time"

	"github.com/enescakir/emoji"
	"github.com/jedib0t/go-pretty/v6/table"

	"github.com/bitcoin-sv/arc/cmd/broadcaster-cli/helper"
	"github.com/bitcoin-sv/arc/internal/broadcaster"
	"github.com/bitcoin-sv/arc/pkg/keyset"
)

type row struct {
	satoshis string
	outputs  string
}

func getUtxosTable(ctx context.Context, logger *slog.Logger, t table.Writer, keySets map[string]*keyset.KeySet, isTestnet bool, wocClient broadcaster.UtxoClient, maxRows int) table.Writer {
	keyTotalOutputs := make([]int, len(keySets))
	keyHeaderRow := make([]interface{}, 0)
	headerRow := make([]interface{}, 0)
	columns := make([][]row, len(keySets))
	maxRowNr := 0
	counter := 0
	names := helper.GetOrderedKeys(keySets)

	for _, name := range names {
		ks := keySets[name]
		utxos, err := wocClient.GetUTXOsWithRetries(ctx, ks.Script, ks.Address(!isTestnet), 1*time.Second, 5)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return t
			}
			logger.Error("failed to get utxos from WoC", slog.String("err", err.Error()))
			continue
		}
		headerRow = append(headerRow, "Sat", "Outputs")
		keyHeaderRow = append(keyHeaderRow, name, "")

		outputsMap := map[uint64]int{}
		var satoshiSlice []uint64
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

		keyTotalOutputs[counter] = getTotalOutputs(counter, satoshiSlice, columns, outputsMap)

		if len(columns[counter]) > maxRowNr {
			maxRowNr = len(columns[counter])
		}

		counter++
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

	return t
}

func getTotalOutputs(counter int, satoshiSlice []uint64, columns [][]row, outputsMap map[uint64]int) int {
	totalOutputs := 0
	for _, satoshi := range satoshiSlice {
		satString := strconv.FormatUint(satoshi, 10)

		if satoshi == 1 {
			satString = satString + " " + emoji.CrossMark.String()
		}

		columns[counter] = append(columns[counter], row{
			satoshis: satString,
			outputs:  strconv.Itoa(outputsMap[satoshi]),
		})

		// Do not count 1-sat outputs, as they can't be used as utxos for transactions
		if satoshi == 1 {
			continue
		}

		totalOutputs += outputsMap[satoshi]
	}
	return totalOutputs
}
