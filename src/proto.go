package lugo

import (
	"encoding/binary"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/xingshuo/lugo/common/seri"
)

//协议格式: 4字节包头长度 + 内容
const PkgHeadLen = 4

type MsgType int

func (mt MsgType) String() string {
	switch mt {
	case PTYPE_TIMER:
		return "TIMER"
	case PTYPE_REQUEST:
		return "REQUEST"
	case PTYPE_RESPONSE:
		return "RESPONSE"
	case PTYPE_CLUSTER_REQ:
		return "CLUSTER_REQ"
	case PTYPE_CLUSTER_RSP:
		return "CLUSTER_RSP"
	default:
		return "unknown"
	}
}

const (
	PTYPE_TIMER       = 1 // 服务定时器消息
	PTYPE_REQUEST     = 2
	PTYPE_RESPONSE    = 3
	PTYPE_CLUSTER_REQ = 4
	PTYPE_CLUSTER_RSP = 5
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

type RpcResponse struct {
	Reply []interface{}
	Err   error
}

type waitNode struct {
	data chan *RpcResponse
	next *waitNode
}

var CLUSTER_REQ_HEAD_LEN = int(new(ClusterReqHead).Size())
var CLUSTER_RSP_HEAD_LEN = int(new(ClusterRspHead).Size())

type ClusterCommonHead struct {
	source      uint64
	session     uint32
	destination uint64
}

func (h *ClusterCommonHead) Pack(b []byte) (uintptr, error) {
	var pos uintptr
	// source
	binary.BigEndian.PutUint64(b[pos:], h.source)
	pos = pos + unsafe.Sizeof(h.source)
	// session
	binary.BigEndian.PutUint32(b[pos:], h.session)
	pos = pos + unsafe.Sizeof(h.session)
	// destination
	binary.BigEndian.PutUint64(b[pos:], h.destination)
	pos = pos + unsafe.Sizeof(h.destination)

	return pos, nil
}

// 这里需要做精确判断
func (h *ClusterCommonHead) Unpack(b []byte) (uintptr, error) {
	var (
		pos     uintptr
		nextPos uintptr
	)
	// source
	nextPos = pos + unsafe.Sizeof(h.source)
	if len(b) < int(nextPos) {
		return pos, PACK_BUFFER_SHORT_ERR
	}
	h.source = binary.BigEndian.Uint64(b[pos:])
	pos = nextPos
	// session
	nextPos = pos + unsafe.Sizeof(h.session)
	if len(b) < int(nextPos) {
		return pos, PACK_BUFFER_SHORT_ERR
	}
	h.session = binary.BigEndian.Uint32(b[pos:])
	pos = nextPos
	// destination
	nextPos = pos + unsafe.Sizeof(h.destination)
	if len(b) < int(nextPos) {
		return pos, PACK_BUFFER_SHORT_ERR
	}
	h.destination = binary.BigEndian.Uint64(b[pos:])
	pos = nextPos

	return pos, nil
}

func (h *ClusterCommonHead) Size() uintptr {
	var size uintptr
	hType := reflect.TypeOf(h)
	for i := 0; i < hType.Elem().NumField(); i++ {
		field := hType.Elem().Field(i)
		size = size + field.Type.Size()
	}
	return size
}

type ClusterReqHead struct {
	ClusterCommonHead
}

func (h *ClusterReqHead) Pack(b []byte) (uintptr, error) {
	// 检查buffer够不够上限,上限会比实际需要用到的多,序列化时buffer一般会足够大
	if len(b) < CLUSTER_REQ_HEAD_LEN {
		return 0, PACK_BUFFER_SHORT_ERR
	}
	return h.ClusterCommonHead.Pack(b)
}

type ClusterRspHead struct {
	ClusterCommonHead
	errCode uint32
}

func (h *ClusterRspHead) Pack(b []byte) (uintptr, error) {
	// 检查buffer够不够上限,上限会比实际需要用到的多,序列化时buffer一般会足够大
	if len(b) < CLUSTER_RSP_HEAD_LEN {
		return 0, PACK_BUFFER_SHORT_ERR
	}
	pos, err := h.ClusterCommonHead.Pack(b)
	if err != nil {
		return pos, err
	}
	binary.BigEndian.PutUint32(b[pos:], h.errCode)
	pos = pos + unsafe.Sizeof(h.errCode)
	return pos, nil
}

func (h *ClusterRspHead) Unpack(b []byte) (uintptr, error) {
	pos, err := h.ClusterCommonHead.Unpack(b)
	if err != nil {
		return pos, err
	}
	nextPos := pos + unsafe.Sizeof(h.errCode)
	if len(b) < int(nextPos) {
		return pos, PACK_BUFFER_SHORT_ERR
	}
	h.errCode = binary.BigEndian.Uint32(b[pos:])
	pos = nextPos
	return pos, nil
}

func (h *ClusterRspHead) Size() uintptr {
	return unsafe.Sizeof(h.errCode) + h.ClusterCommonHead.Size()
}

func NetPackRequest(buffer []byte, source SVC_HANDLE, session uint32, destination SVC_HANDLE, args ...interface{}) ([]byte, error) {
	if PkgHeadLen+1 > len(buffer) {
		return nil, PACK_BUFFER_SHORT_ERR
	}
	buffer[PkgHeadLen] = uint8(PTYPE_CLUSTER_REQ)
	head := &ClusterReqHead{
		ClusterCommonHead{
			source:      uint64(source),
			session:     session,
			destination: uint64(destination)},
	}
	hsize, err := head.Pack(buffer[PkgHeadLen+1:])
	if err != nil {
		return nil, err
	}
	pos := PkgHeadLen + 1 + int(hsize)
	if pos > len(buffer) {
		return nil, PACK_BUFFER_SHORT_ERR
	}
	body := seri.SeriPack(args...)
	if pos+len(body) > len(buffer) {
		return nil, PACK_BUFFER_SHORT_ERR
	}
	bsize := copy(buffer[pos:], body)
	pos += bsize
	binary.BigEndian.PutUint32(buffer, uint32(pos-PkgHeadLen))
	return buffer[:pos], nil
}

func NetPackResponse(buffer []byte, source SVC_HANDLE, session uint32, destination SVC_HANDLE, reply []interface{}, rpcErr error) ([]byte, error) {
	if PkgHeadLen+1 > len(buffer) {
		return nil, PACK_BUFFER_SHORT_ERR
	}
	buffer[PkgHeadLen] = uint8(PTYPE_CLUSTER_RSP)
	errCode := ErrCode_OK
	if rpcErr != nil {
		errCode = ErrCode_NOK
	}
	head := &ClusterRspHead{
		ClusterCommonHead{
			source:      uint64(source),
			session:     session,
			destination: uint64(destination),
		},
		errCode,
	}
	hsize, err := head.Pack(buffer[PkgHeadLen+1:])
	if err != nil {
		return nil, err
	}
	pos := PkgHeadLen + 1 + int(hsize)
	if pos > len(buffer) {
		return nil, PACK_BUFFER_SHORT_ERR
	}

	if rpcErr == nil {
		body := seri.SeriPack(reply...)
		fmt.Printf("pack reply %v to %v", reply, body)
		if pos+len(body) > len(buffer) {
			return nil, PACK_BUFFER_SHORT_ERR
		}
		bsize := copy(buffer[pos:], body)
		pos += bsize
	} else {
		errMsg := rpcErr.Error()
		if pos+len(errMsg) > len(buffer) {
			return nil, PACK_BUFFER_SHORT_ERR
		}
		bsize := copy(buffer[pos:], errMsg)
		pos += bsize
	}
	binary.BigEndian.PutUint32(buffer, uint32(pos-PkgHeadLen))
	return buffer[:pos], nil
}

func NetUnpack(b []byte) (int, []byte) { //返回(消耗字节数,实际内容)
	if len(b) < PkgHeadLen { //不够包头长度
		return 0, nil
	}
	bodyLen := int(binary.BigEndian.Uint32(b))
	if len(b) < PkgHeadLen+bodyLen { //不够body长度
		return 0, nil
	}
	msgLen := PkgHeadLen + bodyLen
	return msgLen, b[PkgHeadLen:msgLen]
}
