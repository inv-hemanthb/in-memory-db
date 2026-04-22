package engine

import (
	"time"

	"github.com/inv-hemanthb/in-memory-db/internal/kv/store"
	"github.com/inv-hemanthb/in-memory-db/internal/transport/tcp"
)

type EngineStatus int

const (
	EngineError EngineStatus = iota
	EngineSuccess
)

type EngineResult struct {
	Status       EngineStatus
	Value        []byte // for GET command
	ErrorMessage string
}

func (engineStatusCode EngineStatus) String() string {
	switch engineStatusCode {
	case EngineError:
		return "ERROR"
	case EngineSuccess:
		return "SUCCESS"
	default:
		return "UNKNOWN"
	}
}

func (engine *EngineResult) Error() string {
	if engine.Status == EngineSuccess {
		return ""
	}
	return engine.ErrorMessage
}

type Engine struct {
	store *store.KVStore
}

func New(kvStore *store.KVStore) *Engine {
	return &Engine{store: kvStore}
}

func (engine *Engine) ExecuteCommand(cmd tcp.Command) EngineResult {
	switch cmd.Type {
	case tcp.CmdSet:
		return engine.handleSet(cmd.Key, cmd.Value, cmd.TTL)
	case tcp.CmdGet:
		return engine.handleGet(cmd.Key)
	case tcp.CmdDelete:
		return engine.handleDelete(cmd.Key)
	case tcp.CmdClear:
		return engine.handleClear()
	default:
		return EngineResult{Status: EngineError, ErrorMessage: "Unknown command"}
	}
}

func (engine *Engine) handleSet(key string, value []byte, ttl *int64) EngineResult {
	if ttl == nil {
		engine.store.Set(key, value, 0)
		return EngineResult{Status: EngineSuccess}
	}
	expiresAt := time.Now().Add(time.Duration(*ttl) * time.Second).UnixNano()
	engine.store.Set(key, value, expiresAt)

	return EngineResult{Status: EngineSuccess}
}

func (engine *Engine) handleGet(key string) EngineResult {
	value, exists := engine.store.Get(key)
	if !exists {
		return EngineResult{Status: EngineError, ErrorMessage: "Key not found"}
	}
	return EngineResult{Status: EngineSuccess, Value: value}
}

func (engine *Engine) handleDelete(key string) EngineResult {
	engine.store.Delete(key)
	return EngineResult{Status: EngineSuccess}

}

func (engine *Engine) handleClear() EngineResult {
	engine.store.Clear()
	return EngineResult{Status: EngineSuccess}
}
