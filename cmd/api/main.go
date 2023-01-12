package main

import (
	"github.com/TAAL-GmbH/arc/cmd"
	"github.com/ordishs/gocore"
)

const progname = "api"

func main() {
	logger := gocore.Log(progname)
	cmd.StartAPIServer(logger)
}
