package models

import (
	"context"
	"testing"

	"github.com/TAAL-GmbH/mapi/client"
	"github.com/labstack/gommon/random"
	"github.com/mrz1836/go-datastore"
	"github.com/stretchr/testify/require"
)

// CreateTestSQLiteClient will create a test client for SQLite
//
// NOTE: you need to close the client using the returned defer func
func CreateTestSQLiteClient(t *testing.T, debug, shared bool, clientOpts ...client.Options) (context.Context, client.Interface, func()) {

	ctx := context.Background()

	// Set the default options, add migrate models
	opts := make([]client.Options, 0)
	opts = append(opts, client.WithSQLite(&datastore.SQLiteConfig{
		CommonConfig: datastore.CommonConfig{
			Debug: debug,
			// random table prefix
			TablePrefix: random.String(16, random.Hex),
		},
		DatabasePath: ":memory:",
		Shared:       shared,
	}))
	opts = append(opts, client.WithMigrateModels(BaseModels))

	if len(clientOpts) > 0 {
		opts = append(opts, clientOpts...)
	}

	// Create the client
	c, err := client.New(opts...)
	require.NoError(t, err)
	require.NotNil(t, c)

	// Create a defer function
	f := func() {
		c.Close()
	}
	return ctx, c, f
}
