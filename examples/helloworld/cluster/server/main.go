package main

import (
	"context"
	"fmt"
	"log"
	"syscall"

	"github.com/xingshuo/lugo/common/seri"

	lugo "github.com/xingshuo/lugo/src"
)

func onReqLogin(ctx context.Context, args ...interface{}) ([]interface{}, error) {
	msg, ok := args[0].(*seri.Table)
	if !ok {
		return nil, fmt.Errorf("proto error")
	}
	svc := lugo.GetSvcFromCtx(ctx)
	session := int64(0)
	svc.SendCluster(ctx, "cluster_client", "gate", "HeartBeat", session, "abcde")
	svc.RegisterTimer(func(ctx context.Context) {
		session++
		svc.SendCluster(ctx, "cluster_client", "gate", "HeartBeat", session, fmt.Sprintf("bcdef%d", session))
	}, 100, 3)
	log.Printf("%s on req login %d", msg.Hashmap["Name"], msg.Hashmap["Gid"])
	return []interface{}{int64(200)}, nil
}

var lobbyMethods = map[string]func(ctx context.Context, args ...interface{}) ([]interface{}, error){
	"ReqLogin": onReqLogin,
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
	server.WaitExit(syscall.SIGINT)
}
