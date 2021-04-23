package main

import (
	"context"
	"fmt"
	"log"
	"syscall"

	lugo "github.com/xingshuo/lugo/src"
)

type ReqLogin struct {
	Gid  uint64
	Name string
}

type RspLogin struct {
	Status int
}

type HeartBeat struct {
	Session int
}

func onReqLogin(ctx context.Context, args ...interface{}) []interface{} {
	msg, ok := args[0].(*ReqLogin)
	if !ok {
		return []interface{}{nil, fmt.Errorf("proto error")}
	}
	svc := lugo.GetSvcFromCtx(ctx)
	session := 500
	svc.SendLua(ctx, "gate", "HeartBeat", &HeartBeat{Session: session})
	svc.RegisterTimer(func() {
		session++
		svc.SendLua(ctx, "gate", "HeartBeat", &HeartBeat{Session: session})
	}, 100, 3)
	log.Printf("%s on req login %d", msg.Name, msg.Gid)
	return []interface{}{&RspLogin{Status: 200}, nil}
}

var lobbyMethods = map[string]func(ctx context.Context, args ...interface{}) []interface{}{
	"ReqLogin": onReqLogin,
}

func onHeartbeat(ctx context.Context, args ...interface{}) []interface{} {
	// svc := sbapi.GetSvcFromCtx(ctx)
	hb, ok := args[0].(*HeartBeat)
	if !ok {
		return []interface{}{nil, fmt.Errorf("proto error")}
	}
	log.Printf("recv heartbeat session %d\n", hb.Session)
	return []interface{}{nil, nil}
}

var gateMethods = map[string]func(ctx context.Context, args ...interface{}) []interface{}{
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
	lobbySvc.RegisterDispatch(lugo.PTYPE_LUA, func(ctx context.Context, args ...interface{}) []interface{} {
		log.Println("lobby args:", args)
		cmd := args[0].(string)
		f := lobbyMethods[cmd]
		return f(ctx, args[1:]...)
	})
	gateSvc, err := server.NewService("gate")
	if err != nil {
		log.Fatalf("new gate service err:%v", err)
	}
	gateSvc.RegisterDispatch(lugo.PTYPE_LUA, func(ctx context.Context, args ...interface{}) []interface{} {
		log.Println("gate args:", args)
		cmd := args[0].(string)
		f := gateMethods[cmd]
		return f(ctx, args[1:]...)
	})
	err, rsp := gateSvc.CallLua(context.Background(), "lobby", "ReqLogin", &ReqLogin{
		Gid:  101,
		Name: "lilei",
	})
	if err == nil {
		log.Printf("rpc result: %d\n", rsp[0].(*RspLogin).Status)
	} else {
		log.Printf("rpc err: %v\n", err)
	}
	server.WaitExit(syscall.SIGINT)
}
