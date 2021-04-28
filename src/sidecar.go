package lugo

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/xingshuo/lugo/common/utils"

	"github.com/xingshuo/lugo/common/netframe"
)

type ClusterSender struct {
	server        *Server
	reqHeadBuffer ClusterReqHead
	rspHeadBuffer ClusterRspHead
}

func (r *ClusterSender) OnConnected(s netframe.Sender) error {
	return nil
}

func (r *ClusterSender) OnMessage(s netframe.Sender, b []byte) (int, error) {
	n, data := NetUnpack(b)
	if n == 0 { // 没解够长度
		return n, nil
	}
	if len(data) == 0 { // 几乎不可能发生
		return n, fmt.Errorf("data is nil")
	}
	msgType := MsgType(data[0])
	// 剔除msgType
	data = data[1:]
	// 跟skynet通信, 会走dialer的reader连接回包
	if msgType == PTYPE_CLUSTER_RSP {
		head := &r.rspHeadBuffer
		pos, err := head.Unpack(data)
		if err != nil {
			return n, err
		}
		dstSvc := r.server.GetServiceByHandle(SVC_HANDLE(head.destination))
		if dstSvc == nil {
			return n, fmt.Errorf("%s not find dst svc %d", msgType, head.destination)
		}
		if head.errCode == ErrCode_OK {
			rsp := &RpcResponse{
				Reply: []interface{}{data[pos:]},
			}
			dstSvc.pushMsg(SVC_HANDLE(head.source), PTYPE_CLUSTER_RSP, head.session, rsp)
		} else {
			rsp := &RpcResponse{
				Err: errors.New(string(data[pos:])),
			}
			dstSvc.pushMsg(SVC_HANDLE(head.source), PTYPE_CLUSTER_RSP, head.session, rsp)
		}
	} else {
		r.server.GetLogSystem().Errorf("recv unknown type %v msg %d", msgType, n)
	}
	return n, nil
}

func (r *ClusterSender) OnClosed(s netframe.Sender) error {
	return nil
}

type ClusterProxy struct {
	server      *Server
	rmtClusters map[string]string           // clustername: address
	hashToNames map[uint32]string           // hashID : clustername
	dialers     map[string]*netframe.Dialer // clustername: dialer
	rwMu        sync.RWMutex
}

func (p *ClusterProxy) Reload(clusterAddrs map[string]string) {
	for name, addr := range p.rmtClusters {
		// 移除失效的Dialer
		if clusterAddrs[name] != addr {
			p.rwMu.Lock()
			d := p.dialers[name]
			delete(p.dialers, name)
			p.rwMu.Unlock()
			if d != nil {
				d.Shutdown()
			}
		}
	}
	p.rmtClusters = clusterAddrs
	// 重新生成hash表
	hashs := make(map[uint32]string)
	for name := range clusterAddrs {
		hashs[utils.ClusterNameToHash(name)] = name
	}
	p.hashToNames = hashs
}

func (p *ClusterProxy) GetDialer(clusterName string) (*netframe.Dialer, error) {
	addr, ok := p.rmtClusters[clusterName]
	if !ok {
		return nil, fmt.Errorf("no such cluster %s", clusterName)
	}
	p.rwMu.Lock()
	defer p.rwMu.Unlock()
	if p.dialers == nil {
		p.dialers = make(map[string]*netframe.Dialer)
	}
	if p.dialers[clusterName] == nil {
		d, err := netframe.NewDialer(addr, func() netframe.Receiver {
			return &ClusterSender{server: p.server}
		})
		if err != nil {
			return nil, err
		}
		err = d.Start()
		if err != nil {
			return nil, err
		}
		p.dialers[clusterName] = d
	}
	return p.dialers[clusterName], nil
}

func (p *ClusterProxy) Exit() {
	p.rwMu.Lock()
	defer p.rwMu.Unlock()
	for _, d := range p.dialers {
		go d.Shutdown()
	}
	p.dialers = nil
}

type GateReceiver struct {
	server        *Server
	reqHeadBuffer ClusterReqHead
	rspHeadBuffer ClusterRspHead
}

func (r *GateReceiver) OnConnected(s netframe.Sender) error {
	return nil
}

func (r *GateReceiver) OnMessage(s netframe.Sender, b []byte) (int, error) {
	n, data := NetUnpack(b)
	if n == 0 { // 没解够长度
		return n, nil
	}
	if len(data) == 0 { // 几乎不可能发生
		return n, fmt.Errorf("data is nil")
	}
	msgType := MsgType(data[0])
	// 剔除msgType
	data = data[1:]
	if msgType == PTYPE_CLUSTER_REQ {
		head := &r.reqHeadBuffer
		pos, err := head.Unpack(data)
		if err != nil {
			return n, err
		}
		dstSvc := r.server.GetServiceByHandle(SVC_HANDLE(head.destination))
		if dstSvc == nil {
			return n, fmt.Errorf("%s not find dst svc %d", msgType, head.destination)
		}
		dstSvc.pushMsg(SVC_HANDLE(head.source), PTYPE_CLUSTER_REQ, head.session, data[pos:])
	} else if msgType == PTYPE_CLUSTER_RSP {
		head := &r.rspHeadBuffer
		pos, err := head.Unpack(data)
		if err != nil {
			return n, err
		}
		dstSvc := r.server.GetServiceByHandle(SVC_HANDLE(head.destination))
		if dstSvc == nil {
			return n, fmt.Errorf("%s not find dst svc %d", msgType, head.destination)
		}
		if head.errCode == ErrCode_OK {
			rsp := &RpcResponse{
				Reply: []interface{}{data[pos:]},
			}
			dstSvc.pushMsg(SVC_HANDLE(head.source), PTYPE_CLUSTER_RSP, head.session, rsp)
		} else {
			rsp := &RpcResponse{
				Err: errors.New(string(data[pos:])),
			}
			dstSvc.pushMsg(SVC_HANDLE(head.source), PTYPE_CLUSTER_RSP, head.session, rsp)
		}
	}
	return n, nil
}

func (r *GateReceiver) OnClosed(s netframe.Sender) error {
	return nil
}

type Sidecar struct {
	server       *Server
	clusterName  string
	gateListener *netframe.Listener
	clusterProxy *ClusterProxy
}

func (sc *Sidecar) Init() error {
	sc.clusterName = sc.server.config.ClusterName
	// 从配置中读取clustername表
	sc.clusterProxy = &ClusterProxy{server: sc.server}
	sc.clusterProxy.Reload(sc.server.config.RemoteAddrs)
	sc.server.GetLogSystem().Info("sidecar reload cluster config done")
	// 绑定本地端口
	l, err := netframe.NewListener(sc.server.config.LocalAddr, func() netframe.Receiver {
		return &GateReceiver{server: sc.server}
	})
	if err != nil {
		return err
	}
	sc.gateListener = l
	sc.server.GetLogSystem().Infof("sidecar listen %s done", sc.server.config.LocalAddr)
	go func() {
		err := l.Serve()
		if err != nil {
			log.Fatalf("gate listener serve err:%v", err)
		} else {
			log.Println("gate listener quit serve")
		}
	}()
	return nil
}

// 更新cluster节点信息
func (sc *Sidecar) Reload() {
	sc.clusterProxy.Reload(sc.server.config.RemoteAddrs)
}

func (sc *Sidecar) Send(clusterName string, data []byte) error {
	d, err := sc.clusterProxy.GetDialer(clusterName)
	if err != nil {
		return err
	}
	return d.Send(data)
}

func (sc *Sidecar) GetClusterName(handle SVC_HANDLE) (string, bool) {
	hashID := uint32(handle >> 32)
	cluster, ok := sc.clusterProxy.hashToNames[hashID]
	if !ok {
		return "unknown", false
	}
	return cluster, true
}

func (sc *Sidecar) Exit() {
	sc.clusterProxy.Exit()
	sc.gateListener.GracefulStop()
}
