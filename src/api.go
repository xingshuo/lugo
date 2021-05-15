package lugo

import (
	"context"
	"log"

	"github.com/xingshuo/lugo/common/utils"
)

func NewServer(config string) (*Server, error) {
	s := &Server{}
	err := s.Init(config)
	if err != nil {
		return nil, err
	}
	s.GetLogSystem().Infof("cluster {%s} start on", s.ClusterName())
	go func() {
		log.Printf("local ip: %s", utils.GetIP())
	}()
	return s, nil
}

func GetSvcFromCtx(ctx context.Context) *Service {
	svc, ok := ctx.Value(CtxKeyService).(*Service)
	if !ok {
		return nil
	}
	return svc
}
