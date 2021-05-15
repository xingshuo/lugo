package lugo

import "context"

type SVC_HANDLE uint64

const (
	DEFAULT_MQ_SIZE  = 1024
	PACK_BUFFER_SIZE = 32767
)

const (
	CtxKeySource     = "LugoSource"
	CtxKeyService    = "LugoService"
	CtxKeyRpcTimeout = "LugoRpcTimeout" // 单位: 10ms
)

type DispatchFunc func(ctx context.Context, args ...interface{}) ([]interface{}, error)
type TimerFunc func(ctx context.Context)
type Timer struct {
	onTick   TimerFunc
	count    int
	interval int
}

type SpawnFunc func(ctx context.Context, args ...interface{})
