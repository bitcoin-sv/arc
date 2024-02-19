package dynamodb

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/bitcoin-sv/arc/blocktx/blocktx_api"
	"github.com/bitcoin-sv/arc/metamorph/metamorph_api"
	"github.com/bitcoin-sv/arc/metamorph/store"
	"github.com/bitcoin-sv/arc/testdata"
	"github.com/libsv/go-p2p/chaincfg/chainhash"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
)

const (
	hostname      = "test-host"
	host          = "http://localhost:"
	regionUsEast1 = "us-east-1"
	port          = "8000/tcp"
)

var (
	dateNow = time.Date(2023, 11, 12, 13, 0, 0, 0, time.UTC)
)

func NewDynamoDBIntegrationTestRepo(t *testing.T) (*DynamoDB, *dynamodb.Client) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	resource, err := pool.Run("amazon/dynamodb-local", "latest", []string{})
	require.NoError(t, err)

	t.Cleanup(func() {
		err := pool.Purge(resource)
		require.NoError(t, err)
	})

	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			PartitionID:   "aws",
			URL:           host + resource.GetPort(port),
			SigningRegion: regionUsEast1,
		}, nil
	})
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithEndpointResolverWithOptions(resolver),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID: "dummy", SecretAccessKey: "dummy", SessionToken: "dummy",
				Source: "Hard-coded credentials; values are irrelevant for local DynamoDB",
			},
		}),
	)
	require.NoError(t, err)

	client := dynamodb.NewFromConfig(cfg)

	pool.MaxWait = 60 * time.Second

	err = pool.Retry(func() error {
		_, err := client.ListTables(context.Background(), &dynamodb.ListTablesInput{})
		return err
	})
	require.NoError(t, err)

	repo, err := New(client, hostname, 1*time.Hour, "test-env", WithNow(func() time.Time { return dateNow }))
	require.NoError(t, err)

	tables, err := client.ListTables(context.Background(), &dynamodb.ListTablesInput{})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"transactions-test-env"}, tables.TableNames)

	return repo, client
}

func putItem(t *testing.T, ctx context.Context, client *dynamodb.Client, storeData any) {
	item, err := attributevalue.MarshalMap(storeData)
	require.NoError(t, err)
	// put item into table
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("transactions-test-env"), Item: item,
	})

	require.NoError(t, err)
}

func TestDynamoDBIntegration(t *testing.T) {

	dataStatusSent := &store.StoreData{
		Hash:          testdata.TX1Hash,
		Status:        metamorph_api.Status_SENT_TO_NETWORK,
		CallbackUrl:   "http://callback.com",
		CallbackToken: "abcd",
		RawTx:         testdata.TX1RawBytes,
		LockedBy:      hostname,
	}

	repo, client := NewDynamoDBIntegrationTestRepo(t)
	ctx := context.Background()

	t.Run("get - error", func(t *testing.T) {
		type invalid struct {
			Hash      *chainhash.Hash `dynamodbav:"tx_hash"`
			WrongType bool            `dynamodbav:"block_height"`
		}
		putItem(t, ctx, client, invalid{
			Hash:      testdata.TX1Hash,
			WrongType: false,
		})

		_, err := repo.Get(ctx, testdata.TX1Hash[:])

		_, isAttrErr := err.(*attributevalue.UnmarshalTypeError)
		require.True(t, isAttrErr)
	})
	t.Run("set/get", func(t *testing.T) {
		err := repo.Set(ctx, nil, dataStatusSent)
		require.NoError(t, err)

		returnedData, err := repo.Get(ctx, testdata.TX1Hash[:])
		require.NoError(t, err)
		require.Equal(t, dataStatusSent, returnedData)
	})

	t.Run("set unlocked", func(t *testing.T) {
		err := repo.SetUnlocked(ctx, []*chainhash.Hash{testdata.TX1Hash})
		require.NoError(t, err)

		returnedData, err := repo.Get(ctx, testdata.TX1Hash[:])
		require.NoError(t, err)
		require.Equal(t, lockedByNone, returnedData.LockedBy)

		dataStatusSent.LockedBy = lockedByNone
	})

	t.Run("get unmined", func(t *testing.T) {
		dataStatusAnnounced := &store.StoreData{
			Hash:     testdata.TX6Hash,
			Status:   metamorph_api.Status_ANNOUNCED_TO_NETWORK,
			RawTx:    testdata.TX6RawBytes,
			LockedBy: lockedByNone,
			StoredAt: dateNow,
		}
		putItem(t, ctx, client, dataStatusAnnounced)

		returnedData, err := repo.GetUnmined(ctx, time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC), 2)
		require.NoError(t, err)
		require.Contains(t, returnedData, dataStatusSent)
		require.Contains(t, returnedData, dataStatusAnnounced)

		tx1, err := repo.Get(ctx, testdata.TX1Hash[:])
		require.NoError(t, err)
		require.Contains(t, hostname, tx1.LockedBy)
		tx2, err := repo.Get(ctx, testdata.TX6Hash[:])
		require.NoError(t, err)
		require.Contains(t, hostname, tx2.LockedBy)
	})

	t.Run("set unlocked by name", func(t *testing.T) {
		results, err := repo.SetUnlockedByName(ctx, "this-does-not-exist")
		require.Equal(t, int64(0), results)
		require.NoError(t, err)

		results, err = repo.SetUnlockedByName(ctx, hostname)
		require.Equal(t, int64(2), results)
		require.NoError(t, err)

		returnedData, err := repo.Get(ctx, testdata.TX1Hash[:])
		require.NoError(t, err)
		require.Equal(t, lockedByNone, returnedData.LockedBy)
		tx2, err := repo.Get(ctx, testdata.TX6Hash[:])
		require.NoError(t, err)
		require.Contains(t, lockedByNone, tx2.LockedBy)
	})

	t.Run("update status", func(t *testing.T) {
		err := repo.UpdateStatus(ctx, testdata.TX1Hash, metamorph_api.Status_REJECTED, "missing inputs")
		require.NoError(t, err)
		returnedDataRejected, err := repo.Get(ctx, testdata.TX1Hash[:])
		require.NoError(t, err)
		require.Equal(t, metamorph_api.Status_REJECTED, returnedDataRejected.Status)
		require.Equal(t, "missing inputs", returnedDataRejected.RejectReason)
		require.Equal(t, testdata.TX1RawBytes, returnedDataRejected.RawTx)

		err = repo.UpdateStatus(ctx, testdata.TX1Hash, metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL, "")
		require.NoError(t, err)
		returnedDataSeenInOrphanMempool, err := repo.Get(ctx, testdata.TX1Hash[:])
		require.NoError(t, err)
		require.Equal(t, metamorph_api.Status_SEEN_IN_ORPHAN_MEMPOOL, returnedDataSeenInOrphanMempool.Status)
		require.Equal(t, testdata.TX1RawBytes, returnedDataSeenInOrphanMempool.RawTx)

		err = repo.UpdateStatus(ctx, testdata.TX1Hash, metamorph_api.Status_SEEN_ON_NETWORK, "")
		require.NoError(t, err)
		returnedDataSeenOnNetwork, err := repo.Get(ctx, testdata.TX1Hash[:])
		require.NoError(t, err)
		require.Equal(t, metamorph_api.Status_SEEN_ON_NETWORK, returnedDataSeenOnNetwork.Status)
		require.Equal(t, testdata.TX1RawBytes, returnedDataSeenOnNetwork.RawTx)

		err = repo.UpdateStatus(ctx, testdata.TX1Hash, metamorph_api.Status_MINED, "")
		require.NoError(t, err)
		returnedDataMined, err := repo.Get(ctx, testdata.TX1Hash[:])
		require.NoError(t, err)
		require.Equal(t, metamorph_api.Status_MINED, returnedDataMined.Status)
		require.Equal(t, dateNow, returnedDataMined.MinedAt)
		require.Equal(t, testdata.TX1RawBytes, returnedDataMined.RawTx)
	})

	t.Run("update status bulk", func(t *testing.T) {
		updates := []store.UpdateStatus{
			{
				Hash:         *testdata.TX1Hash,
				Status:       metamorph_api.Status_REJECTED,
				RejectReason: "missing inputs",
			},
			{
				Hash:   *testdata.TX6Hash,
				Status: metamorph_api.Status_REQUESTED_BY_NETWORK,
			},
		}

		statusUpdates, err := repo.UpdateStatusBulk(ctx, updates)
		require.NoError(t, err)

		require.Equal(t, metamorph_api.Status_REJECTED, statusUpdates[0].Status)
		require.Equal(t, "missing inputs", statusUpdates[0].RejectReason)
		require.Equal(t, testdata.TX1RawBytes, statusUpdates[0].RawTx)

		require.Equal(t, metamorph_api.Status_REQUESTED_BY_NETWORK, statusUpdates[1].Status)
		require.Equal(t, testdata.TX6RawBytes, statusUpdates[1].RawTx)

		returnedDataRejected, err := repo.Get(ctx, testdata.TX1Hash[:])
		require.NoError(t, err)
		require.Equal(t, metamorph_api.Status_REJECTED, returnedDataRejected.Status)
		require.Equal(t, "missing inputs", returnedDataRejected.RejectReason)
		require.Equal(t, testdata.TX1RawBytes, returnedDataRejected.RawTx)

		returnedDataRequested, err := repo.Get(ctx, testdata.TX6Hash[:])
		require.NoError(t, err)
		require.Equal(t, metamorph_api.Status_REQUESTED_BY_NETWORK, returnedDataRequested.Status)
		require.Equal(t, testdata.TX6RawBytes, returnedDataRequested.RawTx)
	})

	t.Run("update mined", func(t *testing.T) {
		txBlocks := &blocktx_api.TransactionBlocks{TransactionBlocks: []*blocktx_api.TransactionBlock{
			{
				BlockHash:       testdata.Block1Hash[:],
				BlockHeight:     100,
				TransactionHash: testdata.TX1Hash[:],
				MerklePath:      "merkle-path-1",
			},
			{
				BlockHash:       testdata.Block1Hash[:],
				BlockHeight:     100,
				TransactionHash: testdata.TX6Hash[:],
				MerklePath:      "merkle-path-2",
			},
		}}

		updated, err := repo.UpdateMined(ctx, txBlocks)
		require.NoError(t, err)
		require.Equal(t, metamorph_api.Status_MINED, updated[0].Status)
		require.Equal(t, testdata.TX1RawBytes, updated[0].RawTx)
		require.Equal(t, dateNow, updated[0].MinedAt)
		require.Equal(t, "merkle-path-1", updated[0].MerklePath)
		require.Equal(t, uint64(100), updated[0].BlockHeight)
		require.True(t, testdata.Block1Hash.IsEqual(updated[0].BlockHash))

		require.Equal(t, metamorph_api.Status_MINED, updated[1].Status)
		require.Equal(t, testdata.TX6RawBytes, updated[1].RawTx)
		require.Equal(t, dateNow, updated[1].MinedAt)
		require.Equal(t, "merkle-path-2", updated[1].MerklePath)
		require.Equal(t, uint64(100), updated[1].BlockHeight)
		require.True(t, testdata.Block1Hash.IsEqual(updated[1].BlockHash))

		returnedData, err := repo.Get(ctx, testdata.TX6Hash[:])
		require.NoError(t, err)
		require.Equal(t, metamorph_api.Status_MINED, returnedData.Status)
		require.Equal(t, testdata.TX6RawBytes, returnedData.RawTx)
		require.Equal(t, dateNow, returnedData.MinedAt)
	})

	t.Run("del", func(t *testing.T) {
		err := repo.Del(ctx, testdata.TX1Hash[:])
		require.NoError(t, err)
		_, err = repo.Get(ctx, testdata.TX1Hash[:])
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("transactions - time to live = -1 hour", func(t *testing.T) {
		repo.ttl = time.Minute * -10
		err := repo.Set(ctx, nil, dataStatusSent)
		require.NoError(t, err)

		time.Sleep(10 * time.Second) // give DynamoDB time to delete
		_, err = repo.Get(ctx, testdata.TX1Hash[:])
		require.ErrorIs(t, err, store.ErrNotFound)
	})
}
