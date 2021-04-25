package lugo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/xingshuo/lugo/common/utils"

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
	svc := s.nameServices[svcName]
	if svc != nil {
		return nil, fmt.Errorf("service re-create")
	}
	handle := SVC_HANDLE(utils.MakeServiceHandle(s.ClusterName(), svcName))
	svc = &Service{
		server: s,
		name:   svcName,
		handle: handle,
	}
	svc.Init()
	s.handleServices[handle] = svc
	s.nameServices[svcName] = svc
	go svc.Serve()
	return svc, nil
}

func (s *Server) DelService(svcName string) {
	s.rwMu.Lock()
	svc := s.nameServices[svcName]
	delete(s.nameServices, svcName)
	if svc != nil {
		delete(s.handleServices, svc.handle)
	}
	s.rwMu.Unlock()
	if svc != nil {
		svc.Exit()
	}
}

func (s *Server) GetService(svcName string) *Service {
	s.rwMu.RLock()
	defer s.rwMu.RUnlock()
	return s.nameServices[svcName]
}

func (s *Server) GetServiceByHandle(handle SVC_HANDLE) *Service {
	s.rwMu.RLock()
	defer s.rwMu.RUnlock()
	return s.handleServices[handle]
}

//接收指定信号，优雅退出接口
func (s *Server) WaitExit(sigs ...os.Signal) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, sigs...)
	sig := <-c
	s.log.Infof("Server(%v) exitNotify with signal(%d)\n", syscall.Getpid(), sig)
	s.Exit()
}

func (s *Server) Exit() {
	s.sidecar.Exit()
	s.rwMu.RLock()
	services := make([]string, 0, len(s.nameServices))
	for name := range s.nameServices {
		services = append(services, name)
	}
	s.rwMu.RUnlock()
	// 顺序退出
	for _, name := range services {
		s.DelService(name)
	}
	s.log.Info("server exit!")
}
