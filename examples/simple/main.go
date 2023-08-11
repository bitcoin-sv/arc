package main

import (
	"encoding/json"
	"fmt"

	"github.com/bitcoin-sv/arc/api"
	apiHandler "github.com/bitcoin-sv/arc/api/handler"
	"github.com/bitcoin-sv/arc/api/transactionHandler"
	"github.com/labstack/echo/v4"
	"github.com/ordishs/go-bitcoin"
	"github.com/ordishs/gocore"
)

func main() {

	// Set up a basic Echo router
	e := echo.New()

	// add a single bitcoin node
	txHandler, err := transactionHandler.NewBitcoinNode("localhost", 8332, "user", "mypassword", false)
	if err != nil {
		panic(err)
	}

	logger := gocore.Log("simple")

	defaultPolicySetting := `
	{"excessiveblocksize":2000000000,"blockmaxsize":512000000,"maxtxsizepolicy":100000000,"maxorphantxsize":1000000000,"datacarriersize":4294967295,"maxscriptsizepolicy":100000000,"maxopsperscriptpolicy":4294967295,"maxscriptnumlengthpolicy":10000,"maxpubkeyspermultisigpolicy":4294967295,"maxtxsigopscountspolicy":4294967295,"maxstackmemoryusagepolicy":100000000,"maxstackmemoryusageconsensus":200000000,"limitancestorcount":10000,"limitcpfpgroupmemberscount":25,"maxmempool":2000000000,"maxmempoolsizedisk":0,"mempoolmaxpercentcpfp":10,"acceptnonstdoutputs":true,"datacarrier":true,"minminingtxfee":1e-8,"maxstdtxvalidationduration":3,"maxnonstdtxvalidationduration":1000,"maxtxchainvalidationbudget":50,"validationclockcpu":true,"minconsolidationfactor":20,"maxconsolidationinputscriptsize":150,"minconfconsolidationinput":6,"minconsolidationinputmaturity":6,"acceptnonstdconsolidationinput":false}`
	var policy *bitcoin.Settings

	err = json.Unmarshal([]byte(defaultPolicySetting), &policy)
	if err != nil {
		panic(err)
	}

	// initialise the arc default api handler, with our txHandler and any handler options
	var handler api.HandlerInterface
	if handler, err = apiHandler.NewDefault(logger, txHandler, policy); err != nil {
		panic(err)
	}

	// Register the ARC API
	// the arc handler registers routes under /v1/...
	api.RegisterHandlers(e, handler)
	// or with a base url => /mySubDir/v1/...
	// arc.RegisterHandlersWithBaseURL(e. blocktx_api, "/arc")

	// Serve HTTP until the world ends.
	e.Logger.Fatal(e.Start(fmt.Sprintf("%s:%d", "0.0.0.0", 8080)))
}
