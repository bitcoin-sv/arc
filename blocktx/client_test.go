package blocktx

//func TestStart(t *testing.T) {
//
//	blockHash, err := hex.DecodeString("0000000000000497b672795cad5e839f63fc43d00ce675836e55338faecc3871")
//	require.NoError(t, err)
//
//	tt := []struct {
//		name         string
//		getStreamErr error
//		recvErr      error
//	}{
//		{
//			name: "success",
//		},
//		{
//			name:         "get stream error",
//			getStreamErr: errors.New("failed to get stream"),
//		},
//		{
//			name:    "recv error",
//			recvErr: errors.New("failed to receive item"),
//		},
//	}
//	for _, tc := range tt {
//		t.Run(tc.name, func(t *testing.T) {
//			block := &blocktx_api.Block{
//				Hash: blockHash,
//			}
//			blockTxClient := new(mock.BlockTxAPIClientMock)
//
//			blockTxClient.On("GetBlockNotificationStream", mock.Anything, mock.Anything).Return(sub, nil)
//
//			//blockTx := &mock.BlockTxAPIClientMock{
//			//	GetBlockNotificationStreamFunc: func(ctx context.Context, in *blocktx_api.Height, opts ...grpc.CallOption) (blocktx_api.BlockTxAPI_GetBlockNotificationStreamClient, error) {
//			//		return &mock.BlockTxAPI_GetBlockNotificationStreamClientMock{
//			//			RecvFunc: func() (*blocktx_api.Block, error) {
//			//				time.Sleep(time.Millisecond * 2)
//			//				return
//			//
//			//			},
//			//		}, tc.getStreamErr
//			//	},
//			//}
//
//			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
//			client := NewClient(blockTx,
//				WithRetryInterval(time.Millisecond*10),
//				WithLogger(logger),
//			)
//
//			client.Start(make(chan *blocktx_api.Block, 10))
//			time.Sleep(time.Millisecond * 15)
//			client.Shutdown()
//
//			require.NoError(t, err)
//		})
//	}
//}
//
