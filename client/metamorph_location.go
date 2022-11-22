package client

import (
	"context"

	"github.com/mrz1836/go-cachestore"
)

type MetamorphLocation interface {
	GetServer(txID string) (string, error)
	SetServer(txID, server string) error
}

type MetamorphLocationCacheService struct {
	ctx        context.Context
	cachestore cachestore.ClientInterface
}

func NewMetamorphLocationCacheService(ctx context.Context, cachestore cachestore.ClientInterface) MetamorphLocation {
	return &MetamorphLocationCacheService{
		ctx:        ctx,
		cachestore: cachestore,
	}
}

func (l *MetamorphLocationCacheService) GetServer(txID string) (string, error) {
	return l.cachestore.Get(l.ctx, l.key(txID))
}

func (l *MetamorphLocationCacheService) SetServer(txID, server string) error {
	return l.cachestore.Set(l.ctx, l.key(txID), server)
}

func (l *MetamorphLocationCacheService) key(txID string) string {
	return "metamorph_location_" + txID
}
