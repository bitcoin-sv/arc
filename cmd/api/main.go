package main

import (
	"github.com/TAAL-GmbH/arc/cmd"
	"github.com/ordishs/gocore"
)

const progname = "api"

func main() {
	logLevel, _ := gocore.Config().Get("logLevel")
	logger := gocore.Log(progname, gocore.NewLogLevelFromString(logLevel))
	cmd.StartAPIServer(logger)
}
