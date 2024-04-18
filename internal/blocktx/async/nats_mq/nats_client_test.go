package nats_mq

import (
	"log"
	"os"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
)

const (
	natsPort = "4222"
)

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("failed to create pool: %v", err)
	}

	opts := dockertest.RunOptions{
		Repository:   "nats",
		Tag:          "2.10.10",
		ExposedPorts: []string{natsPort},
		PortBindings: map[docker.Port][]docker.PortBinding{
			natsPort: {
				{HostIP: "0.0.0.0", HostPort: natsPort},
			},
		},
	}

	resource, err := pool.RunWithOptions(&opts, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
	}

	code := m.Run()

	err = pool.Purge(resource)
	if err != nil {
		log.Fatalf("failed to purge pool: %v", err)
	}

	os.Exit(code)
}

func TestNewNatsClient(t *testing.T) {
	tt := []struct {
		name string
	}{
		{
			name: "success",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewNatsClient("nats://localhost:4222")
			require.NoError(t, err)
		})
	}
}
