package utils

import (
	"hash/fnv"
	"reflect"
	"unsafe"
)

func Assert(condition bool, errMsg string) {
	if !condition {
		panic(errMsg)
	}
}

//handleID生成规则:Hash(clusterName)|Hash(service_name)
func MakeServiceHandle(clusterName, serviceName string) uint64 {
	h := fnv.New32a()
	h.Write([]byte(serviceName))
	lowHash := h.Sum32()

	h = fnv.New32a()
	h.Write([]byte(clusterName))
	highHash := h.Sum32()

	handleID := uint64(highHash)
	handleID = (handleID << 32) | uint64(lowHash)
	return handleID
}

func ClusterNameToHash(clusterName string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(clusterName))
	return h.Sum32()
}

func String2Bytes(s string) []byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	return *(*[]byte)(unsafe.Pointer(&bh))
}

func Bytes2String(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
