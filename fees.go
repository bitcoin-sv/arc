package mapi

import (
	"context"
	"errors"

	"github.com/taal/mapi/client"
	"github.com/taal/mapi/models"
)

// FeeTypes mapi fee types
var FeeTypes = []string{"standard", "data"}

// GetFeesForClient gets fees for a certain client ID, or default fees if no clientID or non found for clientID
func GetFeesForClient(ctx context.Context, c client.Interface, clientID string) ([]Fee, error) {

	fees, err := models.GetFeesForClient(ctx, clientID, models.WithClient(c))
	if err != nil {
		return nil, err
	}

	/*
		if !hasAllFees(fees) {
			defaultFees, err := models.GetDefaultFees(ctx, models.WithClient(c))
		}
	*/

	if len(fees) == 0 {
		return nil, errors.New("fees not found")
	}

	// TODO do something to select the correct fees to return, we might have gotten many records

	mapiFees := make([]Fee, 0)
	for _, f := range fees {
		fee := f
		mapiFees = append(mapiFees, Fee{
			FeeType: &fee.FeeType,
			MiningFee: &FeeAmount{
				Bytes:    &fee.MiningFee.Bytes,
				Satoshis: &fee.MiningFee.Satoshis,
			},
			RelayFee: &FeeAmount{
				Bytes:    &fee.RelayFee.Bytes,
				Satoshis: &fee.RelayFee.Satoshis,
			},
		})
	}

	return mapiFees, nil
}

func hasAllFees(fees []*models.Fee) bool {
	return false
}
