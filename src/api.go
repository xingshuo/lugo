package lugo

import (
	"context"
)

func NewServer(config string) (*Server, error) {
	s := &Server{}
	err := s.Init(config)
	if err != nil {
		return nil, err
	}
	s.GetLogSystem().Infof("cluster {%s} start on", s.ClusterName())
	return s, nil
}

func GetSvcFromCtx(ctx context.Context) *Service {
	svc, ok := ctx.Value(CtxKeyService).(*Service)
	if !ok {
		return nil
	}
	return svc
}
