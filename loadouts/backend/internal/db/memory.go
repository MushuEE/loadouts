package db

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gmccloskey/loadouts/backend/internal/core"
)

type MemoryStore struct {
	mu            sync.RWMutex
	schemas       map[string]core.SchemaDefinition
	items         map[string]core.Item
	userMetadata map[string]core.UserMetadata
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		schemas:       make(map[string]core.SchemaDefinition),
		items:         make(map[string]core.Item),
		userMetadata: make(map[string]core.UserMetadata),
	}
}

func (s *MemoryStore) CreateSchema(ctx context.Context, schema core.SchemaDefinition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s:%s", schema.ID, schema.Version)
	s.schemas[key] = schema
	return nil
}

func (s *MemoryStore) GetSchema(ctx context.Context, id, version string) (core.SchemaDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", id, version)
	schema, ok := s.schemas[key]
	if !ok {
		return core.SchemaDefinition{}, fmt.Errorf("schema not found")
	}
	return schema, nil
}

func (s *MemoryStore) ListSchemas(ctx context.Context) ([]core.SchemaDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.SchemaDefinition, 0, len(s.schemas))
	for _, v := range s.schemas {
		list = append(list, v)
	}
	return list, nil
}

func (s *MemoryStore) CreateItem(ctx context.Context, item core.Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return nil
}

func (s *MemoryStore) GetItem(ctx context.Context, id string) (core.Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	if !ok {
		return core.Item{}, fmt.Errorf("item not found")
	}
	return item, nil
}

func (s *MemoryStore) ListItems(ctx context.Context, query string) ([]core.Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]core.Item, 0)
	for _, v := range s.items {
		if query == "" || strings.Contains(strings.ToLower(v.Name), strings.ToLower(query)) {
			list = append(list, v)
		}
	}
	return list, nil
}

func (s *MemoryStore) UpdateUserMetadata(ctx context.Context, meta core.UserMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s:%s", meta.UserID, meta.ItemID)
	s.userMetadata[key] = meta
	return nil
}

func (s *MemoryStore) GetUserMetadata(ctx context.Context, userID, itemID string) (*core.UserMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", userID, itemID)
	meta, ok := s.userMetadata[key]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return &meta, nil
}
