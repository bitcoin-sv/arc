package models

import (
	"context"
	"fmt"

	"github.com/mrz1836/go-datastore"
)

// Save will save the model(s) into the Datastore
func Save(ctx context.Context, model ModelInterface) (err error) {

	// Check for a client
	c := model.Client()
	if c == nil {
		return fmt.Errorf("missing client")
	}

	// Check for a datastore
	ds := c.Datastore()
	if ds == nil {
		return fmt.Errorf("missing datastore")
	}

	return model.Client().Datastore().NewTx(ctx, func(tx *datastore.Transaction) error {
		// update the timestamps on the db record
		model.SetRecordTime(model.IsNew())
		if err = model.Client().Datastore().SaveModel(ctx, model, tx, model.IsNew(), true); err != nil {
			return err
		}

		// rest the new flag, if applicable
		model.NotNew()
		return nil
	})
}
