package cache

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"

	"github.com/bitcoin-sv/arc/pkg/test_utils"
)

var (
	hostPort    int
	redisClient *redis.Client
)

const (
	port = 6379
)

func TestMain(m *testing.M) {
	flag.Parse()

	if testing.Short() {
		os.Exit(0)
	}

	os.Exit(testmain(m))
}

func testmain(m *testing.M) int {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Printf("failed to create pool: %v", err)
		return 1
	}

	resource, resourcePort, err := testutils.RunRedis(pool, strconv.Itoa(port), "cache")
	if err != nil {
		log.Print(err)
		return 1
	}
	defer func() {
		err = pool.Purge(resource)
		if err != nil {
			log.Fatalf("failed to purge pool: %v", err)
		}
	}()

	hostPort, err = strconv.Atoi(resourcePort)
	if err != nil {
		log.Fatalf("failed to convert port to int: %v", err)
	}

	return m.Run()
}

func setup() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("localhost:%d", hostPort),
		Password: "",
		DB:       1,
	})

	_, err := redisClient.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalln(err)
	}
}

func TestRedisClient(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	setup()

	redisStore := NewRedisStore(ctx, redisClient)

	t.Run("get/set", func(t *testing.T) {
		// given
		err := redisStore.Set("key", []byte("value"), 4*time.Second)
		require.NoError(t, err)

		// when
		res, err := redisStore.Get("key")
		require.NoError(t, err)
		// then
		require.Equal(t, "value", string(res))
		//when
		res, err = redisStore.Get("NonExistingKey")
		//then
		require.ErrorIs(t, err, ErrCacheNotFound)
		require.Nil(t, res)
	})

	t.Run("del", func(t *testing.T) {
		// given
		err := redisStore.Set("key1", []byte("value1"), 4*time.Second)
		require.NoError(t, err)
		err = redisStore.Set("key2", []byte("value2"), 4*time.Second)
		require.NoError(t, err)
		err = redisStore.Set("key3", []byte("value3"), 4*time.Second)
		require.NoError(t, err)

		// when
		err = redisStore.Del([]string{"key1", "key2", "key3"}...)
		// then
		require.NoError(t, err)

		// when
		err = redisStore.Del([]string{"nonExistingKey"}...)
		// then
		require.ErrorIs(t, err, ErrCacheNotFound)
	})

	t.Run("map set/get", func(t *testing.T) {
		// when
		err := redisStore.MapSet("hash", "key1", []byte("value1"))
		require.NoError(t, err)

		res, err := redisStore.MapGet("hash", "key1")
		require.NoError(t, err)

		// then
		require.Equal(t, "value1", string(res))
	})

	t.Run("map del", func(t *testing.T) {
		// given
		err := redisStore.MapSet("hash", "key1", []byte("value1"))
		require.NoError(t, err)
		err = redisStore.MapSet("hash", "key2", []byte("value2"))
		require.NoError(t, err)
		err = redisStore.MapSet("hash", "key3", []byte("value3"))
		require.NoError(t, err)

		// when/then
		err = redisStore.MapDel("hash", []string{"key1", "key2"}...)
		require.NoError(t, err)

		err = redisStore.MapDel("hash", "key3")
		require.NoError(t, err)
	})

	t.Run("map get all", func(t *testing.T) {
		// given
		err := redisStore.MapSet("hash", "key1", []byte("value1"))
		require.NoError(t, err)
		err = redisStore.MapSet("hash", "key2", []byte("value2"))
		require.NoError(t, err)
		err = redisStore.MapSet("hash", "key3", []byte("value3"))
		require.NoError(t, err)

		// when
		res, err := redisStore.MapGetAll("hash")
		require.NoError(t, err)

		// then
		expectedResponse := map[string][]byte{"key1": []byte("value1"), "key2": []byte("value2"), "key3": []byte("value3")}
		require.Equal(t, expectedResponse, res)

		err = redisStore.Del("hash")
		require.NoError(t, err)
	})

	t.Run("map len", func(t *testing.T) {
		// given
		err := redisStore.MapSet("hash", "key1", []byte("value1"))
		require.NoError(t, err)
		err = redisStore.MapSet("hash", "key2", []byte("value2"))
		require.NoError(t, err)
		err = redisStore.MapSet("hash", "key3", []byte("value3"))
		require.NoError(t, err)

		// when
		res, err := redisStore.MapLen("hash")
		require.NoError(t, err)

		// then
		require.Equal(t, int64(3), res)

		err = redisStore.Del("hash")
		require.NoError(t, err)
	})

	t.Run("map extract all", func(t *testing.T) {
		// given
		err := redisStore.MapSet("hash", "key1", []byte("value1"))
		require.NoError(t, err)
		err = redisStore.MapSet("hash", "key2", []byte("value2"))
		require.NoError(t, err)
		err = redisStore.MapSet("hash", "key3", []byte("value3"))
		require.NoError(t, err)

		// when
		res, err := redisStore.MapExtractAll("hash")
		require.NoError(t, err)

		// then
		expectedResponse := map[string][]byte{"key1": []byte("value1"), "key2": []byte("value2"), "key3": []byte("value3")}
		require.Equal(t, expectedResponse, res)

		hLen, err := redisStore.MapLen("hash")
		require.NoError(t, err)
		require.Equal(t, int64(0), hLen)
	})
}
