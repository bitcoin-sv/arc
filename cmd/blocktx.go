package cmd

import (
	"github.com/TAAL-GmbH/arc/blocktx"
	"github.com/TAAL-GmbH/arc/blocktx/store/sql"
	"github.com/ordishs/gocore"
)

func StartBlockTx(logger *gocore.Logger) {
	blockStore, err := sql.NewSQLStore("sqlite")
	if err != nil {
		panic("Could not connect to fn: " + err.Error())
	}

	blockNotifier := blocktx.NewBlockNotifier(blockStore, logger)

	blockTxServer := blocktx.NewServer(blockStore, blockNotifier, logger)

	if err = blockTxServer.StartGRPCServer(); err != nil {
		logger.Fatal(err)
	}
}
