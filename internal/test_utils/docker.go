package testutils

import (
	"errors"
	"fmt"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"

	_ "github.com/golang-migrate/migrate/v4/source/file" //nolint: revive // Required for migrations
)

const (
	dbName     = "main_test"
	dbUsername = "arcuser"
	dbPassword = "arcpass"
)

func RunAndMigratePostgresql(pool *dockertest.Pool, port, migrationTable, migrationsPath string) (*dockertest.Resource, string, error) {
	resource, dbInfo, err := RunPostgresql(pool, port)
	if err != nil {
		return nil, "", fmt.Errorf("failed run postgresql: %v", err)
	}

	err = MigrateUp(migrationTable, migrationsPath, dbInfo)
	if err != nil {
		pErr := pool.Purge(resource)
		if pErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to purge pool: %v", pErr))
		}
		return nil, "", fmt.Errorf("failed to run migration: %v", err)
	}

	return resource, dbInfo, nil
}

func RunPostgresql(pool *dockertest.Pool, port string) (*dockertest.Resource, string, error) {
	opts := dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "15.4",
		Env: []string{
			fmt.Sprintf("POSTGRES_PASSWORD=%s", dbPassword),
			fmt.Sprintf("POSTGRES_USER=%s", dbUsername),
			fmt.Sprintf("POSTGRES_DB=%s", dbName),
			"listen_addresses = '*'",
		},
		ExposedPorts: []string{"5432"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"5432": {
				{HostIP: "0.0.0.0", HostPort: port},
			},
		},
	}

	resource, err := pool.RunWithOptions(&opts, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
		config.Tmpfs = map[string]string{
			"/var/lib/postgresql/data": "",
		}
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to create resource: %v", err)
	}

	hostPort := resource.GetPort("5432/tcp")
	dbInfo := fmt.Sprintf("host=localhost port=%s user=%s password=%s dbname=%s sslmode=disable", hostPort, dbUsername, dbPassword, dbName)
	return resource, dbInfo, nil
}

func RunNats(pool *dockertest.Pool, port, name string, cmds ...string) (*dockertest.Resource, string, error) {
	opts := dockertest.RunOptions{
		Repository:   "nats",
		Tag:          "2.10.10",
		ExposedPorts: []string{"4222"},
		PortBindings: map[docker.Port][]docker.PortBinding{
			"4222": {
				{HostIP: "0.0.0.0", HostPort: port},
			},
		},
		Name: name,
		Cmd:  cmds,
	}
	resource, err := pool.RunWithOptions(&opts, func(config *docker.HostConfig) {
		// set AutoRemove to true so that stopped container goes away by itself
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to create resource: %v", err)
	}

	hostPort := resource.GetPort("4222/tcp")
	natsURL := fmt.Sprintf("nats://localhost:%s", hostPort)

	return resource, natsURL, nil
}
