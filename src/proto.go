package lugo

import "fmt"

type MsgType int

func (mt MsgType) String() string {
	switch mt {
	case PTYPE_RESPONSE:
		return "RESPONSE"
	case PTYPE_LUA:
		return "LUA"
	default:
		return "unknown"
	}
}

const (
	PTYPE_TIMER    = -1 // 服务定时器消息, 这里和skynet不同
	PTYPE_RESPONSE = 1
	PTYPE_LUA      = 10
)

type Message struct {
	Source  SVC_HANDLE
	MsgType MsgType
	Session uint32
	Data    []interface{}
}

func (m *Message) String() string {
	return fmt.Sprintf("msg:[%d,%s,%d]", m.Source, m.MsgType, m.Session)
}

type protocol struct {
	name     string
	msgType  MsgType
	pack     func(args ...interface{}) []byte
	unpack   func(buffer []byte) []interface{}
	dispatch DispatchFunc
}

type protoMngr struct {
	nameProtos map[string]*protocol
	typeProtos map[MsgType]*protocol
}

func (p *protoMngr) Register(proto *protocol) {
	if p.typeProtos[proto.msgType] != nil {
		panic(fmt.Sprintf("re-register proto %d", proto.msgType))
	}
	if p.nameProtos[proto.name] != nil {
		panic(fmt.Sprintf("re-register proto %s", proto.name))
	}
	p.nameProtos[proto.name] = proto
	p.typeProtos[proto.msgType] = proto
}

type RpcResponse struct {
	Reply []interface{}
	Err   error
}

type waitNode struct {
	data chan *RpcResponse
	next *waitNode
}
