package lugo

import (
	"context"
	"fmt"
	"time"

	"github.com/xingshuo/lugo/common/lib"
	"github.com/xingshuo/lugo/common/log"
)

type Service struct {
	// new初始化
	server *Server
	name   string // 服务名
	handle SVC_HANDLE
	// Init初始化
	ctx          context.Context
	protoMngr    *protoMngr
	mqueue       *MsgQueue
	msgNotify    chan struct{}
	exitNotify   *lib.SyncEvent
	exitDone     *lib.SyncEvent
	log          *log.LogSystem
	sessionSeq   uint32
	suspend      chan struct{}
	waitSessions map[uint32]chan *RpcResponse
	waitPool     *waitPool
	timerSeq     uint32
	timers       map[uint32]*Timer
}

func (s *Service) String() string {
	return fmt.Sprintf("[%s-%d]", s.name, s.handle)
}

func (s *Service) Init() {
	s.ctx = context.WithValue(context.Background(), CtxKeyService, s)
	s.protoMngr = &protoMngr{
		nameProtos: make(map[string]*protocol),
		typeProtos: make(map[MsgType]*protocol),
	}
	s.protoMngr.Register(&protocol{
		name:    "LUA",
		msgType: PTYPE_LUA,
	})
	s.mqueue = NewMQueue(DEFAULT_MQ_SIZE)
	s.msgNotify = make(chan struct{}, 1)
	s.exitNotify = lib.NewSyncEvent()
	s.exitDone = lib.NewSyncEvent()
	s.log = s.server.GetLogSystem()
	s.suspend = make(chan struct{}, 1)
	s.waitSessions = make(map[uint32]chan *RpcResponse)
	s.waitPool = &waitPool{}
	s.waitPool.init()
	s.timers = make(map[uint32]*Timer)
}

// 服务自定义logger实现
func (s *Service) SetLogSystem(logger log.Logger, lv log.LogLevel) {
	s.log = log.NewLogSystem(logger, lv)
}

func (s *Service) GetLogSystem() *log.LogSystem {
	return s.log
}

func (s *Service) NewTimerSeq() uint32 {
	for {
		s.timerSeq++
		if s.timerSeq == 0 {
			s.timerSeq++
		}
		if s.timers[s.timerSeq] != nil {
			continue
		}
		return s.timerSeq
	}
}

func (s *Service) NewRpcSeq() uint32 {
	for {
		s.sessionSeq++
		if s.sessionSeq == 0 {
			s.sessionSeq++
		}
		if s.waitSessions[s.sessionSeq] != nil {
			continue
		}
		return s.sessionSeq
	}
}

// 节点内Notify
func (s *Service) Send(ctx context.Context, svcName string, msgType MsgType, data ...interface{}) error {
	ds := s.server.GetService(svcName)
	if ds == nil {
		return fmt.Errorf("unknown dst svc %s", svcName)
	}
	ds.pushMsg(s.handle, msgType, 0, data...)
	return nil
}

func (s *Service) Call(ctx context.Context, svcName string, msgType MsgType, data ...interface{}) (error, []interface{}) {
	ds := s.server.GetService(svcName)
	if ds == nil {
		return fmt.Errorf("unknown dst svc %s", svcName), nil
	}
	session := s.NewRpcSeq()
	done := s.waitPool.get()
	s.waitSessions[session] = done
	ds.pushMsg(s.handle, msgType, session, data...)
	// 通知Serve继续处理其他消息
	s.suspend <- struct{}{}
	timeout, _ := ctx.Value(CtxKeyRpcTimeout).(int)
	if timeout > 0 {
		time.AfterFunc(time.Duration(timeout)*time.Millisecond*10, func() {
			s.pushMsg(0, PTYPE_RESPONSE, session, &RpcResponse{
				Err: RPC_TIMEOUT_ERR,
			})
		})
	}
	rsp := <-done
	s.waitPool.put(done)
	return rsp.Err, rsp.Reply
}

func (s *Service) SendLua(ctx context.Context, svcName string, data ...interface{}) error {
	return s.Send(ctx, svcName, PTYPE_LUA, data...)
}

func (s *Service) CallLua(ctx context.Context, svcName string, data ...interface{}) (error, []interface{}) {
	return s.Call(ctx, svcName, PTYPE_LUA, data...)
}

// interval:执行间隔, 单位: 10毫秒 (和skynet保持一致)
// 注意: interval == 0时, 定时消息立即回射, 且固定只执行一次. 典型应用场景: 服务初始化
// count: 执行次数, > 0:有限次, == 0:无限次
func (s *Service) RegisterTimer(onTick TimerFunc, interval int, count int) uint32 {
	seq := s.NewTimerSeq()
	t := &Timer{
		onTick:   onTick,
		count:    count,
		interval: interval,
	}
	s.timers[seq] = t
	// 立即回射
	if t.interval == 0 {
		t.count = 1
		s.pushMsg(0, PTYPE_TIMER, seq)
	} else {
		if t.interval <= 1 {
			t.interval = 1
		}
		time.AfterFunc(time.Duration(t.interval)*time.Millisecond*10, func() {
			s.pushMsg(0, PTYPE_TIMER, seq)
		})
	}
	return seq
}

func (s *Service) onTimeout(seq uint32) {
	t := s.timers[seq]
	if t != nil {
		t.onTick()
		if t.count > 0 { // 有限次执行
			t.count--
			if t.count == 0 {
				delete(s.timers, seq)
			} else {
				time.AfterFunc(time.Duration(t.interval)*time.Millisecond*10, func() {
					s.pushMsg(0, PTYPE_TIMER, seq)
				})
			}
		} else {
			time.AfterFunc(time.Duration(t.interval)*time.Millisecond*10, func() {
				s.pushMsg(0, PTYPE_TIMER, seq)
			})
		}
	} else {
		s.log.Errorf("unknown timer seq %d", seq)
	}
	s.suspend <- struct{}{}
}

func (s *Service) pushMsg(source SVC_HANDLE, msgType MsgType, session uint32, data ...interface{}) {
	wakeUp := s.mqueue.Push(source, msgType, session, data)
	if wakeUp {
		select {
		case s.msgNotify <- struct{}{}:
		default:
		}
	}
}

func (s *Service) RegisterDispatch(msgType MsgType, dispatch DispatchFunc) error {
	p := s.protoMngr.typeProtos[msgType]
	if p == nil {
		s.log.Errorf("RegisterDispatch err unknown msg type %v", msgType)
		return fmt.Errorf("register unknown proto type %v", msgType)
	}
	p.dispatch = dispatch
	return nil
}

func (s *Service) rawDispatch(f DispatchFunc, source SVC_HANDLE, session uint32, msg ...interface{}) {
	defer func() {
		s.suspend <- struct{}{}
		/*		if e := recover(); e != nil {
				s.log.Errorf("panic occurred on recv svc req: %v", e)
			}*/
	}()
	reply := f(context.WithValue(s.ctx, CtxKeySource, source), msg...)
	if session != 0 {
		srcSvc := s.server.GetServiceByHandle(source)
		if srcSvc != nil {
			srcSvc.pushMsg(s.handle, PTYPE_RESPONSE, session, &RpcResponse{
				Reply: reply,
			})
		} else {
			s.log.Errorf("unknown src service %d", source)
		}
	}
}

func (s *Service) onResponse(session uint32, rsp *RpcResponse) {
	done := s.waitSessions[session]
	if done == nil {
		s.log.Errorf("rpc wakeup no exit session %d", session)
		s.suspend <- struct{}{}
		return
	}
	delete(s.waitSessions, session)
	select {
	// 根据session唤醒, 这里不需要唤醒suspend chan, 等发起rpc的goroutine处理完自己唤醒
	case done <- rsp:
	default:
		s.log.Errorf("rpc wakeup session %d failed", session)
		s.suspend <- struct{}{}
	}
}

func (s *Service) dispatchMsg(source SVC_HANDLE, msgType MsgType, session uint32, msg ...interface{}) {
	s.log.Infof("dispatch msg is %v", msg)
	if msgType == PTYPE_RESPONSE {
		if len(msg) != 1 {
			s.log.Errorf("response msg len err %v", msg)
			return
		}
		rsp, ok := msg[0].(*RpcResponse)
		if !ok {
			s.log.Errorf("response msg type err %v", msg)
			return
		}
		go s.onResponse(session, rsp)
	} else if msgType == PTYPE_TIMER {
		go s.onTimeout(session)
	} else {
		p := s.protoMngr.typeProtos[msgType]
		if p == nil {
			s.log.Errorf("unknown msg type %v", msgType)
			return
		}
		if p.dispatch == nil {
			s.log.Errorf("no dispatch msg type %v", msgType)
			return
		}
		go s.rawDispatch(p.dispatch, source, session, msg...)
	}

	<-s.suspend
	s.log.Debugf("%s dispatch %s done from %s", s, msgType, s.server.GetServiceByHandle(source))
}

func (s *Service) Serve() {
	s.log.Infof("cluster %s new service %s", s.server.ClusterName(), s)
	for {
		select {
		case <-s.msgNotify:
			for {
				empty, source, msgType, session, data := s.mqueue.Pop()
				if empty {
					break
				}
				s.dispatchMsg(source, msgType, session, data...)
			}
		case <-s.exitNotify.Done():
			for {
				empty, source, msgType, session, data := s.mqueue.Pop()
				if empty {
					break
				}
				s.dispatchMsg(source, msgType, session, data...)
			}
			s.exitDone.Fire()
			return
		}
	}
}

func (s *Service) Exit() {
	if s.exitNotify.Fire() {
		<-s.exitDone.Done()
	}
	s.log.Infof("service %s exit!\n", s)
}
