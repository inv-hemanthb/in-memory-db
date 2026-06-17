package api

import (
	"context"
	"errors"
	"fmt"

	apidb "github.com/inv-hemanthb/in-memory-db/internal/api/db"
	"github.com/inv-hemanthb/in-memory-db/internal/api/kvclient"
)

type ItemService struct {
	store *apidb.Store
	kv    *kvclient.Client
}

type OpResult struct {
	Item     apidb.Item
	CacheHit *bool
}

func NewItemService(store *apidb.Store, kv *kvclient.Client) *ItemService {
	return &ItemService{store: store, kv: kv}
}

func cacheKey(id int64) string {
	return fmt.Sprintf("item:%d", id)
}

func (s *ItemService) Create(ctx context.Context, withKV bool, key, value string) (OpResult, error) {
	item, err := s.store.Create(ctx, key, value)
	if err != nil {
		return OpResult{}, err
	}

	if withKV {
		if err := s.kv.Set(ctx, cacheKey(item.ID), []byte(item.Value)); err != nil {
			return OpResult{}, err
		}
	}

	return OpResult{Item: item}, nil
}

func (s *ItemService) ReadByID(ctx context.Context, withKV bool, id int64) (OpResult, error) {
	if !withKV {
		item, err := s.store.GetByID(ctx, id)
		if err != nil {
			return OpResult{}, err
		}
		return OpResult{Item: item}, nil
	}

	kvKey := cacheKey(id)
	cached, err := s.kv.Get(ctx, kvKey)
	if err == nil {
		hit := true
		return OpResult{
			Item:     apidb.Item{ID: id, Value: string(cached)},
			CacheHit: &hit,
		}, nil
	}
	if !errors.Is(err, kvclient.ErrNotFound) {
		return OpResult{}, err
	}

	item, err := s.store.GetByID(ctx, id)
	if err != nil {
		return OpResult{}, err
	}

	if err := s.kv.Set(ctx, kvKey, []byte(item.Value)); err != nil {
		return OpResult{}, err
	}

	miss := false
	return OpResult{Item: item, CacheHit: &miss}, nil
}

func (s *ItemService) ReadByKey(ctx context.Context, withKV bool, key string) (OpResult, error) {
	item, err := s.store.GetByKey(ctx, key)
	if err != nil {
		return OpResult{}, err
	}

	if !withKV {
		return OpResult{Item: item}, nil
	}

	kvKey := cacheKey(item.ID)
	cached, err := s.kv.Get(ctx, kvKey)
	if err == nil {
		hit := true
		item.Value = string(cached)
		return OpResult{Item: item, CacheHit: &hit}, nil
	}
	if !errors.Is(err, kvclient.ErrNotFound) {
		return OpResult{}, err
	}

	if err := s.kv.Set(ctx, kvKey, []byte(item.Value)); err != nil {
		return OpResult{}, err
	}

	miss := false
	return OpResult{Item: item, CacheHit: &miss}, nil
}

func (s *ItemService) ReadByIDAndKey(ctx context.Context, withKV bool, id int64, key string) (OpResult, error) {
	if !withKV {
		item, err := s.store.GetByIDAndKey(ctx, id, key)
		if err != nil {
			return OpResult{}, err
		}
		return OpResult{Item: item}, nil
	}

	kvKey := cacheKey(id)
	cached, err := s.kv.Get(ctx, kvKey)
	if err == nil {
		hit := true
		return OpResult{
			Item:     apidb.Item{ID: id, Key: key, Value: string(cached)},
			CacheHit: &hit,
		}, nil
	}
	if !errors.Is(err, kvclient.ErrNotFound) {
		return OpResult{}, err
	}

	item, err := s.store.GetByIDAndKey(ctx, id, key)
	if err != nil {
		return OpResult{}, err
	}

	if err := s.kv.Set(ctx, kvKey, []byte(item.Value)); err != nil {
		return OpResult{}, err
	}

	miss := false
	return OpResult{Item: item, CacheHit: &miss}, nil
}

func (s *ItemService) Update(ctx context.Context, withKV bool, id int64, value string) (OpResult, error) {
	item, err := s.store.Update(ctx, id, value)
	if err != nil {
		return OpResult{}, err
	}

	if withKV {
		if err := s.kv.Set(ctx, cacheKey(item.ID), []byte(item.Value)); err != nil {
			return OpResult{}, err
		}
	}

	return OpResult{Item: item}, nil
}

func (s *ItemService) Delete(ctx context.Context, withKV bool, id int64, hard bool) error {
	var err error
	if hard {
		err = s.store.HardDelete(ctx, id)
	} else {
		err = s.store.SoftDelete(ctx, id)
	}
	if err != nil {
		return err
	}

	if withKV {
		if err := s.kv.Delete(ctx, cacheKey(id)); err != nil {
			return err
		}
	}

	return nil
}

func (s *ItemService) ClearCache(ctx context.Context) error {
	return s.kv.Clear(ctx)
}
