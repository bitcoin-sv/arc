//go:build e2e

package test

// func TestDoubleSpend(t *testing.T) {
// 	t.Run("submit tx with a double spend tx before and after tx got mined - ext format", func(t *testing.T) {
// 		address, privateKey := node_client.FundNewWallet(t, bitcoind)

// 		utxos := node_client.GetUtxos(t, bitcoind, address)
// 		require.True(t, len(utxos) > 0, "No UTXOs available for the address")

// 		callbackURL, token, callbackReceivedChan, callbackErrChan, cleanup := CreateCallbackServer(t)
// 		defer cleanup()

// 		tx1, err := node_client.CreateTx(privateKey, address, utxos[0])
// 		require.NoError(t, err)

// 		// submit first transaction
// 		rawTx1, err := tx1.EFHex()
// 		require.NoError(t, err)
// 		t.Logf("tx 1 ID: %s", tx1.TxID().String())

// 		resp := postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx1}),
// 			map[string]string{
// 				"X-WaitFor":       StatusSeenOnNetwork,
// 				"X-CallbackUrl":   callbackURL,
// 				"X-CallbackToken": token,
// 			},
// 			http.StatusOK)
// 		require.Equal(t, StatusSeenOnNetwork, resp.TxStatus)

// 		// send double spending transaction when first tx is in mempool
// 		tx2 := createTxToNewAddress(t, privateKey, utxos[0])
// 		rawTx2, err := tx2.EFHex()
// 		require.NoError(t, err)
// 		t.Logf("tx 2 ID: %s", tx2.TxID().String())

// 		// submit second transaction
// 		resp = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx2}),
// 			map[string]string{
// 				"X-WaitFor":       StatusDoubleSpendAttempted,
// 				"X-CallbackUrl":   callbackURL,
// 				"X-CallbackToken": token,
// 			},
// 			http.StatusOK)
// 		require.Equal(t, StatusDoubleSpendAttempted, resp.TxStatus)
// 		require.ElementsMatch(t, []string{tx1.TxID().String()}, *resp.CompetingTxs)

// 		tx3 := createTxToNewAddress(t, privateKey, utxos[0])
// 		rawTx3, err := tx3.EFHex()
// 		require.NoError(t, err)
// 		t.Logf("tx 3 ID: %s", tx3.TxID().String())

// 		resp = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTx3}),
// 			map[string]string{
// 				"X-WaitFor":       StatusDoubleSpendAttempted,
// 				"X-CallbackUrl":   callbackURL,
// 				"X-CallbackToken": token,
// 			},
// 			http.StatusOK)
// 		require.Equal(t, StatusDoubleSpendAttempted, resp.TxStatus)
// 		//require.ElementsMatch(t, []string{tx1.TxID().String(), tx2.TxID().String()}, *resp.CompetingTxs)

// 		// give arc time to update the status of all competing transactions
// 		time.Sleep(5 * time.Second)

// 		statusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
// 		statusResp := getRequest[TransactionResponse](t, statusURL)

// 		// verify that the first tx was also set to DOUBLE_SPEND_ATTEMPTED
// 		require.Equal(t, StatusDoubleSpendAttempted, statusResp.TxStatus)
// 		require.ElementsMatch(t, []string{tx3.TxID().String(), tx2.TxID().String()}, *statusResp.CompetingTxs)
// 		//require.Equal(t, []string{tx2.TxID().String()}, *statusResp.CompetingTxs)

// 		// mine the first tx
// 		node_client.Generate(t, bitcoind, 1)

// 		// verify that one of the competing transactions was mined, and the other was rejected
// 		tx1StatusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx1.TxID())
// 		tx1StatusResp := getRequest[TransactionResponse](t, tx1StatusURL)

// 		tx2StatusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx2.TxID())
// 		tx2StatusResp := getRequest[TransactionResponse](t, tx2StatusURL)

// 		tx3StatusURL := fmt.Sprintf("%s/%s", arcEndpointV1Tx, tx3.TxID())
// 		tx3StatusResp := getRequest[TransactionResponse](t, tx3StatusURL)

// 		require.Contains(t, []string{tx1StatusResp.TxStatus, tx2StatusResp.TxStatus, tx3StatusResp.TxStatus}, StatusMined)
// 		type callbackData struct {
// 			txID   string
// 			status string
// 		}

// 		var expectedReceivedCallbacks []callbackData

// 		if tx1StatusResp.TxStatus == StatusMined {
// 			checkDoubleSpendResponse(t, tx1StatusResp, tx2StatusResp, tx3StatusResp)
// 			expectedReceivedCallbacks = append(expectedReceivedCallbacks, callbackData{txID: tx1.TxID().String(), status: StatusMined})
// 			expectedReceivedCallbacks = append(expectedReceivedCallbacks, callbackData{txID: tx2.TxID().String(), status: StatusRejected})
// 			expectedReceivedCallbacks = append(expectedReceivedCallbacks, callbackData{txID: tx3.TxID().String(), status: StatusRejected})
// 		} else if tx2StatusResp.TxStatus == StatusMined {
// 			checkDoubleSpendResponse(t, tx2StatusResp, tx1StatusResp, tx3StatusResp)
// 			expectedReceivedCallbacks = append(expectedReceivedCallbacks, callbackData{txID: tx2.TxID().String(), status: StatusMined})
// 			expectedReceivedCallbacks = append(expectedReceivedCallbacks, callbackData{txID: tx1.TxID().String(), status: StatusRejected})
// 			expectedReceivedCallbacks = append(expectedReceivedCallbacks, callbackData{txID: tx3.TxID().String(), status: StatusRejected})
// 		} else if tx3StatusResp.TxStatus == StatusMined {
// 			checkDoubleSpendResponse(t, tx3StatusResp, tx2StatusResp, tx1StatusResp)
// 			expectedReceivedCallbacks = append(expectedReceivedCallbacks, callbackData{txID: tx3.TxID().String(), status: StatusMined})
// 			expectedReceivedCallbacks = append(expectedReceivedCallbacks, callbackData{txID: tx1.TxID().String(), status: StatusRejected})
// 			expectedReceivedCallbacks = append(expectedReceivedCallbacks, callbackData{txID: tx2.TxID().String(), status: StatusRejected})
// 		}

// 		// wait for callbacks
// 		callbackTimeout := time.After(5 * time.Second)

// 		var receivedCallbacks []callbackData

// 	callbackLoop:
// 		for {
// 			select {
// 			case status := <-callbackReceivedChan:
// 				receivedCallbacks = append(receivedCallbacks, callbackData{txID: status.Txid, status: status.TxStatus})
// 			case err = <-callbackErrChan:
// 				t.Fatalf("callback error: %v", err)
// 			case <-callbackTimeout:
// 				break callbackLoop
// 			}
// 		}

// 		t.Log("expected callbacks", expectedReceivedCallbacks)
// 		t.Log("received callbacks", receivedCallbacks)
// 		require.ElementsMatch(t, expectedReceivedCallbacks, receivedCallbacks)

// 		// send double spending transaction when previous tx was mined
// 		txMined := createTxToNewAddress(t, privateKey, utxos[0])
// 		rawTxMined, err := txMined.EFHex()
// 		require.NoError(t, err)

// 		resp = postRequest[TransactionResponse](t, arcEndpointV1Tx, createPayload(t, TransactionRequest{RawTx: rawTxMined}), map[string]string{"X-WaitFor": StatusSeenInOrphanMempool}, http.StatusOK)
// 		require.Equal(t, StatusSeenInOrphanMempool, resp.TxStatus)
// 	})
// }

// func checkDoubleSpendResponse(t *testing.T, minedResp TransactionResponse, rejectedResponses ...TransactionResponse) {
// 	require.Equal(t, *minedResp.ExtraInfo, "previously double spend attempted")
// 	for _, rejectedResp := range rejectedResponses {
// 		require.Equal(t, StatusRejected, rejectedResp.TxStatus)
// 		require.Equal(t, *rejectedResp.ExtraInfo, "double spend attempted")
// 	}
// }

// func createTxToNewAddress(t *testing.T, privateKey string, utxo node_client.UnspentOutput) *sdkTx.Transaction {
// 	netCfg := chaincfg.TestNet

// 	newKeyset, err := keyset.New(&netCfg)
// 	require.NoError(t, err)

// 	newAddress := newKeyset.Address(false)
// 	tx1, err := node_client.CreateTx(privateKey, newAddress, utxo)
// 	require.NoError(t, err)

// 	return tx1
// }
