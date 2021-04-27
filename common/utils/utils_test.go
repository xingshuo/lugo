package utils

import (
	"fmt"
	"hash/fnv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFnvHash(t *testing.T) {
	clusterName := "zonesvr1"
	svcName := "lobby"
	fullName := fmt.Sprintf("%s.%s", clusterName, svcName)

	h := fnv.New32()
	h.Write([]byte(clusterName))
	assert.Equal(t, uint32(2744007411), h.Sum32())

	h = fnv.New32()
	h.Write([]byte(svcName))
	assert.Equal(t, uint32(290285391), h.Sum32())

	h = fnv.New32()
	h.Write([]byte(fullName))
	assert.Equal(t, uint32(1578440609), h.Sum32())
}
