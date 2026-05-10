package service

import (
	"context"
	"log"
	"sync"

	"github.com/bits-and-blooms/bloom/v3"
	"github.com/gmccloskey/loadouts/backend/internal/db"
)

type ItemBloomFilter struct {
	mu     sync.RWMutex
	filter *bloom.BloomFilter
}

func NewItemBloomFilter(estimatedItems uint, falsePositiveRate float64) *ItemBloomFilter {
	return &ItemBloomFilter{
		filter: bloom.NewWithEstimates(estimatedItems, falsePositiveRate),
	}
}

func (bf *ItemBloomFilter) Add(id string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	bf.filter.Add([]byte(id))
}

func (bf *ItemBloomFilter) Test(id string) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	return bf.filter.Test([]byte(id))
}

// PopulateFromDB scans the database for existing item IDs and adds them to the filter.
func (bf *ItemBloomFilter) PopulateFromDB(ctx context.Context, store db.Store) error {
	items, err := store.ListItems(ctx, "") // Empty query to get all
	if err != nil {
		return err
	}

	bf.mu.Lock()
	defer bf.mu.Unlock()
	for _, item := range items {
		bf.filter.Add([]byte(item.ID))
	}
	log.Printf("Populated Bloom Filter with %d existing items", len(items))
	return nil
}
