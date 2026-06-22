package api

import (
	"context"
	"errors"
	"fmt"

	apidb "github.com/inv-hemanthb/in-memory-db/internal/api/db"
	"github.com/inv-hemanthb/in-memory-db/internal/api/kvclient"
	"github.com/inv-hemanthb/in-memory-db/internal/logger"
)

type ItemService struct {
	store *apidb.Store
	kv    *kvclient.Client
	log   *logger.Logger
}

type OpResult struct {
	Item     apidb.Item
	CacheHit *bool
}

func NewItemService(store *apidb.Store, kv *kvclient.Client, log *logger.Logger) *ItemService {
	return &ItemService{store: store, kv: kv, log: log}
}

func cacheKey(id int64) string {
	return fmt.Sprintf("item:%d", id)
}

func (s *ItemService) Create(ctx context.Context, withKV bool, key, value string) (OpResult, error) {
	item, err := s.store.Create(ctx, key, value)
	if err != nil {
		s.log.Error("PG INSERT key=%s: %v", key, err)
		return OpResult{}, err
	}
	s.log.Trace("PG INSERT key=%s id=%d", key, item.ID)

	if withKV {
		kvKey := cacheKey(item.ID)
		if err := s.kv.Set(ctx, kvKey, []byte(item.Value)); err != nil {
			s.log.Error("KV SET %s: %v", kvKey, err)
			return OpResult{}, err
		}
		s.log.Trace("KV SET %s", kvKey)
	} else {
		s.log.Trace("cache skipped (with_kv=false)")
	}

	return OpResult{Item: item}, nil
}

func (s *ItemService) ReadByID(ctx context.Context, withKV bool, id int64) (OpResult, error) {
	if !withKV {
		item, err := s.store.GetByID(ctx, id)
		if err != nil {
			s.log.Error("PG SELECT id=%d: %v", id, err)
			return OpResult{}, err
		}
		s.log.Trace("PG SELECT id=%d", id)
		return OpResult{Item: item}, nil
	}

	kvKey := cacheKey(id)
	cached, err := s.kv.Get(ctx, kvKey)
	if err == nil {
		s.log.Trace("KV GET %s hit", kvKey)
		hit := true
		return OpResult{
			Item:     apidb.Item{ID: id, Value: string(cached)},
			CacheHit: &hit,
		}, nil
	}
	if !errors.Is(err, kvclient.ErrNotFound) {
		s.log.Error("KV GET %s: %v", kvKey, err)
		return OpResult{}, err
	}

	s.log.Trace("KV GET %s miss", kvKey)

	item, err := s.store.GetByID(ctx, id)
	if err != nil {
		s.log.Error("PG SELECT id=%d: %v", id, err)
		return OpResult{}, err
	}
	s.log.Trace("PG SELECT id=%d", id)

	if err := s.kv.Set(ctx, kvKey, []byte(item.Value)); err != nil {
		s.log.Error("KV SET %s: %v", kvKey, err)
		return OpResult{}, err
	}
	s.log.Trace("KV SET %s", kvKey)

	miss := false
	return OpResult{Item: item, CacheHit: &miss}, nil
}

func (s *ItemService) ReadByKey(ctx context.Context, withKV bool, key string) (OpResult, error) {
	item, err := s.store.GetByKey(ctx, key)
	if err != nil {
		s.log.Error("PG SELECT key=%s: %v", key, err)
		return OpResult{}, err
	}
	s.log.Trace("PG SELECT key=%s id=%d", key, item.ID)

	if !withKV {
		return OpResult{Item: item}, nil
	}

	kvKey := cacheKey(item.ID)
	cached, err := s.kv.Get(ctx, kvKey)
	if err == nil {
		s.log.Trace("KV GET %s hit", kvKey)
		hit := true
		item.Value = string(cached)
		return OpResult{Item: item, CacheHit: &hit}, nil
	}
	if !errors.Is(err, kvclient.ErrNotFound) {
		s.log.Error("KV GET %s: %v", kvKey, err)
		return OpResult{}, err
	}

	s.log.Trace("KV GET %s miss", kvKey)

	if err := s.kv.Set(ctx, kvKey, []byte(item.Value)); err != nil {
		s.log.Error("KV SET %s: %v", kvKey, err)
		return OpResult{}, err
	}
	s.log.Trace("KV SET %s", kvKey)

	miss := false
	return OpResult{Item: item, CacheHit: &miss}, nil
}

func (s *ItemService) ReadByIDAndKey(ctx context.Context, withKV bool, id int64, key string) (OpResult, error) {
	if !withKV {
		item, err := s.store.GetByIDAndKey(ctx, id, key)
		if err != nil {
			s.log.Error("PG SELECT id=%d key=%s: %v", id, key, err)
			return OpResult{}, err
		}
		s.log.Trace("PG SELECT id=%d key=%s", id, key)
		return OpResult{Item: item}, nil
	}

	kvKey := cacheKey(id)
	cached, err := s.kv.Get(ctx, kvKey)
	if err == nil {
		s.log.Trace("KV GET %s hit", kvKey)
		hit := true
		return OpResult{
			Item:     apidb.Item{ID: id, Key: key, Value: string(cached)},
			CacheHit: &hit,
		}, nil
	}
	if !errors.Is(err, kvclient.ErrNotFound) {
		s.log.Error("KV GET %s: %v", kvKey, err)
		return OpResult{}, err
	}

	s.log.Trace("KV GET %s miss", kvKey)

	item, err := s.store.GetByIDAndKey(ctx, id, key)
	if err != nil {
		s.log.Error("PG SELECT id=%d key=%s: %v", id, key, err)
		return OpResult{}, err
	}
	s.log.Trace("PG SELECT id=%d key=%s", id, key)

	if err := s.kv.Set(ctx, kvKey, []byte(item.Value)); err != nil {
		s.log.Error("KV SET %s: %v", kvKey, err)
		return OpResult{}, err
	}
	s.log.Trace("KV SET %s", kvKey)

	miss := false
	return OpResult{Item: item, CacheHit: &miss}, nil
}

func (s *ItemService) Update(ctx context.Context, withKV bool, id int64, value string) (OpResult, error) {
	item, err := s.store.Update(ctx, id, value)
	if err != nil {
		s.log.Error("PG UPDATE id=%d: %v", id, err)
		return OpResult{}, err
	}
	s.log.Trace("PG UPDATE id=%d", id)

	kvKey := cacheKey(item.ID)
	if withKV {
		if err := s.kv.Set(ctx, kvKey, []byte(item.Value)); err != nil {
			s.log.Error("KV SET %s: %v", kvKey, err)
			return OpResult{}, err
		}
		s.log.Trace("KV SET %s", kvKey)
	} else {
		if err := s.kv.Delete(ctx, kvKey); err != nil {
			s.log.Error("KV DELETE %s: %v", kvKey, err)
			return OpResult{}, err
		}
		s.log.Trace("KV DELETE %s (invalidate)", kvKey)
	}

	return OpResult{Item: item}, nil
}

func (s *ItemService) Delete(ctx context.Context, withKV bool, id int64, hard bool) error {
	var err error
	if hard {
		err = s.store.HardDelete(ctx, id)
		s.log.Trace("PG DELETE id=%d hard", id)
	} else {
		err = s.store.SoftDelete(ctx, id)
		s.log.Trace("PG DELETE id=%d soft", id)
	}
	if err != nil {
		s.log.Error("PG DELETE id=%d: %v", id, err)
		return err
	}

	kvKey := cacheKey(id)
	if err := s.kv.Delete(ctx, kvKey); err != nil {
		s.log.Error("KV DELETE %s: %v", kvKey, err)
		return err
	}
	if withKV {
		s.log.Trace("KV DELETE %s", kvKey)
	} else {
		s.log.Trace("KV DELETE %s (invalidate)", kvKey)
	}

	return nil
}

func (s *ItemService) ClearCache(ctx context.Context) error {
	if err := s.kv.Clear(ctx); err != nil {
		s.log.Error("KV CLEAR: %v", err)
		return err
	}
	s.log.Trace("KV CLEAR")
	return nil
}
