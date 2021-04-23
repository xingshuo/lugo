package lugo

import "fmt"

var (
	RPC_SESSION_REPEAT_ERR  = fmt.Errorf("rpc Session repeat")
	RPC_SESSION_NOEXIST_ERR = fmt.Errorf("rpc Session no exist")
	RPC_TIMEOUT_ERR         = fmt.Errorf("rpc timeout")
	RPC_WAKEUP_ERR          = fmt.Errorf("rpc wake up err")
)
