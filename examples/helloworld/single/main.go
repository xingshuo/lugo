package main

import (
	"context"
	"fmt"
	"log"
	"syscall"

	"github.com/xingshuo/lugo/common/seri"

	lugo "github.com/xingshuo/lugo/src"
)

type HeartBeat struct {
	Session int
}

func onReqLogin(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	msg, ok := args[0].(*seri.Table)
	if !ok {
		return nil, fmt.Errorf("proto error")
	}
	svc := lugo.GetSvcFromCtx(ctx)
	session := int64(0)
	svc.Send(ctx, "gate", "HeartBeat", session)
	svc.RegisterTimer(func() {
		session++
		svc.Send(ctx, "gate", "HeartBeat", session)
	}, 100, 3)
	log.Printf("%s on req login %d", msg.Hashmap["Name"], msg.Hashmap["Gid"])
	return []interface{}{int64(200)}, nil
}

var lobbyMethods = map[string]func(ctx context.Context, args ...interface{}) ([]interface{}, error){
	"ReqLogin": onReqLogin,
}

func onHeartbeat(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	// svc := sbapi.GetSvcFromCtx(ctx)
	session, ok := args[0].(int64)
	if !ok {
		return nil, fmt.Errorf("proto error")
	}
	log.Printf("recv heartbeat session %d\n", session)
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
	// server.GetLogSystem().SetLevel(sblog.LevelDebug)
	lobbySvc, err := server.NewService("lobby")
	if err != nil {
		log.Fatalf("new lobby service err:%v", err)
	}
	lobbySvc.RegisterDispatch(func(ctx context.Context, args ...interface{}) ([]interface{}, error) {
		log.Println("lobby args:", args)
		cmd := args[0].(string)
		f := lobbyMethods[cmd]
		return f(ctx, args[1:]...)
	})
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
	rsp, err := gateSvc.Call(context.Background(), "lobby", "ReqLogin", &seri.Table{
		Hashmap: map[interface{}]interface{}{
			"Gid":  int64(101),
			"Name": "lilei",
		},
	})
	if err == nil {
		log.Printf("rpc result: %d\n", rsp[0].(int))
	} else {
		log.Printf("rpc err: %v\n", err)
	}
	server.WaitExit(syscall.SIGINT)
}
