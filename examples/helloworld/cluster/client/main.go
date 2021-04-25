package main

import (
	"context"
	"fmt"
	"log"
	"syscall"

	"github.com/xingshuo/lugo/common/seri"

	lugo "github.com/xingshuo/lugo/src"
)

func onHeartbeat(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// svc := sbapi.GetSvcFromCtx(ctx)
	session, ok := args[0].(int64)
	if !ok {
		return nil, fmt.Errorf("session error")
	}
	token, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("token error")
	}
	log.Printf("recv heartbeat session %d token %s\n", session, token)
	return nil, nil
}

var gateMethods = map[string]func(ctx context.Context, args ...interface{}) ([]interface{}, error){
	"HeartBeat": onHeartbeat,
}

func main() {
	server, err := lugo.NewServer("config.json")
	if err != nil {
		log.Fatalf("new server err:%v", err)
	}
	gateSvc, err := server.NewService("gate")
	if err != nil {
		log.Fatalf("new gate service err:%v", err)
	}
	gateSvc.RegisterDispatch(func(ctx context.Context, args ...interface{}) ([]interface{}, error) {
		log.Println("gate args:", args)
		cmd := args[0].(string)
		f := gateMethods[cmd]
		return f(ctx, args[1:]...)
	})
	rsp, err := gateSvc.CallCluster(context.Background(), "cluster_server", "lobby", "ReqLogin", &seri.Table{
		Hashmap: map[interface{}]interface{}{
			"Gid":  int64(101),
			"Name": "lilei",
		},
	})
	if err == nil {
		log.Printf("rpc result: %d\n", rsp[0].(int64))
	} else {
		log.Printf("rpc err: %v\n", err)
	}
	server.WaitExit(syscall.SIGINT)
}
