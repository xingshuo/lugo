package lugo

import (
	"encoding/json"
	"io/ioutil"
	"sync"
	"sync/atomic"

	"github.com/xingshuo/lugo/common/log"
)

type ServerConfig struct {
	ClusterName string            // 当前节点名
	LocalAddr   string            // 本进程/容器 mesh地址 ip:port
	RemoteAddrs map[string]string // 远端节点地址表
}

type Server struct {
	config         ServerConfig
	rwMu           sync.RWMutex
	handleServices map[SVC_HANDLE]*Service
	nameServices   map[string]*Service
	sidecar        *Sidecar
	log            *log.LogSystem
	handleSeq      uint64
}

func (s *Server) Init(config string) error {
	s.handleServices = make(map[SVC_HANDLE]*Service)
	s.nameServices = make(map[string]*Service)
	err := s.loadConfig(config)
	if err != nil {
		return err
	}
	s.sidecar = &Sidecar{server: s}
	err = s.sidecar.Init()
	if err != nil {
		return err
	}
	s.log = log.NewStdLogSystem(log.LevelInfo)
	return nil
}

func (s *Server) NewSvcHandle() SVC_HANDLE {
	for {
		handle := SVC_HANDLE(atomic.AddUint64(&s.handleSeq, 1))
		if handle == 0 {
			continue
		}
		if s.handleServices[handle] != nil {
			continue
		}
		return handle
	}
	return 0
}

func (s *Server) ClusterName() string {
	return s.config.ClusterName
}

func (s *Server) SetLogSystem(logger log.Logger, lv log.LogLevel) {
	s.log = log.NewLogSystem(logger, lv)
}

func (s *Server) GetLogSystem() *log.LogSystem {
	return s.log
}

func (s *Server) loadConfig(config string) error {
	data, err := ioutil.ReadFile(config)
	if err != nil {
		s.log.Errorf("load config %s failed:%v\n", config, err)
		return err
	}
	err = json.Unmarshal(data, &s.config)
	if err != nil {
		s.log.Errorf("load config %s failed:%v.\n", config, err)
		return err
	}
	return nil
}

func (s *Server) NewService(svcName string) (*Service, error) {
	s.rwMu.Lock()
	defer s.rwMu.Unlock()
	handle := s.NewSvcHandle()

}
