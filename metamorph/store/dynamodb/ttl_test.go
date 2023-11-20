package dynamodb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/ory/dockertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeToLive(t *testing.T) {
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
		context.TODO(),
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

	_, err = New(client, hostname)
	require.NoError(t, err)

	tables, err := client.ListTables(context.Background(), &dynamodb.ListTablesInput{})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"blocks", "transactions"}, tables.TableNames)
	testkey := []byte("testkey")

	//sdata := &store.StoreData{
	//	Hash:          TX1Hash,
	//	Status:        metamorph_api.Status_SENT_TO_NETWORK,
	//	CallbackUrl:   "http://callback.com",
	//	CallbackToken: "abcd",
	//	MerkleProof:   false,
	//	RawTx:         TX1RawBytes,
	//	LockedBy:      hostname,
	//	StoredAt:      time.Now(),
	//}
	//require.NoError(t, repo.Set(context.TODO(), testkey, sdata))
	//_, err = repo.Get(context.TODO(), testkey)
	// this test should not pass. Get should return the item that was inserted previously
	//assert.Equal(t, store.ErrNotFound, err)

	_, err = client.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String("transactions"),
		Item: map[string]types.AttributeValue{
			"tx_hash":   &types.AttributeValueMemberB{Value: testkey},
			"locked_by": &types.AttributeValueMemberS{Value: "testlocked"},
			"ttl":       &types.AttributeValueMemberS{Value: "1"},
		},
	})
	require.NoError(t, err)

	getitem, err := client.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String("transactions"),
		Key: map[string]types.AttributeValue{
			"tx_hash": &types.AttributeValueMemberB{Value: testkey},
		}})
	require.NoError(t, err)

	assert.Equal(t, "testlocked", getitem.Item["locked_by"].(*types.AttributeValueMemberS).Value)

	time.Sleep(10 * time.Second)
	result, err := client.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: aws.String("transactions"),
		Key: map[string]types.AttributeValue{
			"tx_hash": &types.AttributeValueMemberB{Value: testkey},
		}})

	require.NotNil(t, err)
	fmt.Println(result)

}
