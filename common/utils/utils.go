package utils

import (
	"hash/fnv"
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
