package metamorph_test

//func TestNewProcessorResponseMap(t *testing.T) {
//	t.Run("default", func(t *testing.T) {
//		expiry := 10 * time.Second
//		prm := NewProcessorResponseMap(expiry)
//		assert.Equalf(t, expiry, prm.Expiry, "NewProcessorResponseMap(%v)", expiry)
//		assert.Len(t, prm.ResponseItems, 0)
//	})
//}
//
//func TestProcessorResponseMapGet(t *testing.T) {
//	tt := []struct {
//		name string
//		hash *chainhash.Hash
//
//		expectedFound bool
//	}{
//		{
//			name: "found",
//			hash: testdata.TX1Hash,
//
//			expectedFound: true,
//		},
//		{
//			name: "not found",
//			hash: testdata.TX2Hash,
//
//			expectedFound: false,
//		},
//	}
//
//	for _, tc := range tt {
//		t.Run(tc.name, func(t *testing.T) {
//			proc := NewProcessorResponseMap(50 * time.Millisecond)
//			proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_SENT_TO_NETWORK))
//
//			item, ok := proc.Get(tc.hash)
//			require.Equal(t, tc.expectedFound, ok)
//
//			if tc.expectedFound {
//				require.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, item.GetStatus())
//			}
//		})
//	}
//}
//
//func TestProcessorResponseMap_Clear(t *testing.T) {
//	t.Run("default", func(t *testing.T) {
//		proc := NewProcessorResponseMap(50 * time.Millisecond)
//		proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_SENT_TO_NETWORK))
//		item, ok := proc.Get(testdata.TX1Hash)
//		assert.True(t, ok)
//		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, item.GetStatus())
//
//		proc.Clear()
//
//		item, ok = proc.Get(testdata.TX1Hash)
//		assert.False(t, ok)
//		assert.Nil(t, item)
//	})
//}
//
//func TestProcessorResponseMap_Delete(t *testing.T) {
//	t.Run("default", func(t *testing.T) {
//		proc := NewProcessorResponseMap(2 * time.Second)
//		proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_SENT_TO_NETWORK))
//		proc.Set(testdata.TX2Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX2Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK))
//
//		item, ok := proc.Get(testdata.TX1Hash)
//		require.True(t, ok)
//		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, item.GetStatus())
//
//		item2, ok2 := proc.Get(testdata.TX2Hash)
//		require.True(t, ok2)
//		assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, item2.GetStatus())
//
//		proc.Delete(testdata.TX1Hash)
//
//		item, ok = proc.Get(testdata.TX1Hash)
//		require.False(t, ok)
//		assert.Nil(t, item)
//
//		item2, ok2 = proc.Get(testdata.TX2Hash)
//		require.True(t, ok2)
//		assert.Equal(t, metamorph_api.Status_ANNOUNCED_TO_NETWORK, item2.GetStatus())
//	})
//}
//
//func TestProcessorResponseMap_Get(t *testing.T) {
//	t.Run("empty", func(t *testing.T) {
//		proc := NewProcessorResponseMap(2 * time.Second)
//		_, ok := proc.Get(testdata.TX1Hash)
//		require.False(t, ok)
//	})
//
//	t.Run("returned", func(t *testing.T) {
//		proc := NewProcessorResponseMap(2 * time.Second)
//		proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_SENT_TO_NETWORK))
//
//		item, ok := proc.Get(testdata.TX1Hash)
//		require.True(t, ok)
//		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, item.GetStatus())
//	})
//
//	t.Run("status returned", func(t *testing.T) {
//		tests := []struct {
//			status metamorph_api.Status
//			ok     bool
//		}{
//			{status: metamorph_api.Status_UNKNOWN, ok: true},
//			{status: metamorph_api.Status_QUEUED, ok: true},
//			{status: metamorph_api.Status_RECEIVED, ok: true},
//			{status: metamorph_api.Status_STORED, ok: true},
//			{status: metamorph_api.Status_ANNOUNCED_TO_NETWORK, ok: true},
//			{status: metamorph_api.Status_SENT_TO_NETWORK, ok: true},
//			{status: metamorph_api.Status_SEEN_ON_NETWORK, ok: true},
//			{status: metamorph_api.Status_MINED, ok: true},
//			{status: metamorph_api.Status_CONFIRMED, ok: true},
//			{status: metamorph_api.Status_REJECTED, ok: true},
//		}
//		for _, tt := range tests {
//			proc := NewProcessorResponseMap(2 * time.Second)
//			proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, tt.status))
//
//			item, ok := proc.Get(testdata.TX1Hash)
//			require.Equal(t, ok, tt.ok)
//			if tt.ok {
//				assert.Equal(t, tt.status, item.GetStatus())
//			}
//		}
//	})
//}
//
//func TestProcessorResponseMap_Items(t *testing.T) {
//	t.Run("empty", func(t *testing.T) {
//		proc := NewProcessorResponseMap(2 * time.Second)
//		assert.Len(t, proc.Items(), 0)
//	})
//
//	t.Run("default", func(t *testing.T) {
//		proc := NewProcessorResponseMap(2 * time.Second)
//		proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_SENT_TO_NETWORK))
//		proc.Set(testdata.TX2Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX2Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK))
//
//		items := proc.Items()
//		assert.Equal(t, 2, len(items))
//		assert.Equal(t, testdata.TX1Hash, items[*testdata.TX1Hash].Hash)
//		assert.Equal(t, testdata.TX2Hash, items[*testdata.TX2Hash].Hash)
//	})
//}
//
//func TestProcessorResponseMap_Len(t *testing.T) {
//	t.Run("empty", func(t *testing.T) {
//		proc := NewProcessorResponseMap(2 * time.Second)
//		assert.Equal(t, 0, proc.Len())
//	})
//
//	t.Run("default", func(t *testing.T) {
//		proc := NewProcessorResponseMap(2 * time.Second)
//		proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_SENT_TO_NETWORK))
//		proc.Set(testdata.TX2Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX2Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK))
//
//		assert.Equal(t, 2, proc.Len())
//	})
//}
//
//func TestProcessorResponseMap_Set(t *testing.T) {
//	t.Run("empty key", func(t *testing.T) {
//		proc := NewProcessorResponseMap(2 * time.Second)
//		proc.Set(&chainhash.Hash{}, processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_SENT_TO_NETWORK))
//		assert.Len(t, proc.ResponseItems, 1)
//
//		item := proc.ResponseItems[chainhash.Hash{}]
//		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, item.GetStatus())
//	})
//
//	t.Run("default", func(t *testing.T) {
//		proc := NewProcessorResponseMap(2 * time.Second)
//		assert.Len(t, proc.ResponseItems, 0)
//
//		proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_SENT_TO_NETWORK))
//
//		assert.Len(t, proc.ResponseItems, 1)
//
//		item := proc.ResponseItems[*testdata.TX1Hash]
//		assert.Equal(t, metamorph_api.Status_SENT_TO_NETWORK, item.GetStatus())
//	})
//}
//
//func TestProcessorResponseMap_clean(t *testing.T) {
//	t.Run("default", func(t *testing.T) {
//		proc := NewProcessorResponseMap(2 * time.Second)
//		proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_SENT_TO_NETWORK))
//		proc.Set(testdata.TX2Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX2Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK))
//
//		proc.Clean()
//
//		assert.Equal(t, 2, proc.Len())
//	})
//
//	t.Run("Expiry", func(t *testing.T) {
//		proc := NewProcessorResponseMap(50 * time.Millisecond)
//		proc.Set(testdata.TX1Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX1Hash, metamorph_api.Status_SENT_TO_NETWORK))
//		proc.Set(testdata.TX2Hash, processor_response.NewProcessorResponseWithStatus(testdata.TX2Hash, metamorph_api.Status_ANNOUNCED_TO_NETWORK))
//
//		time.Sleep(100 * time.Millisecond)
//
//		proc.Clean()
//
//		assert.Equal(t, 0, proc.Len())
//	})
//}
