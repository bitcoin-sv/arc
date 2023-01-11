package main

import (
	"github.com/TAAL-GmbH/arc/cmd"
	"github.com/ordishs/gocore"
)

const progname = "api"

var logger = gocore.Log(progname)

func main() {
	cmd.StartAPIServer(logger)
}
