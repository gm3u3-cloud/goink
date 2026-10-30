package testsupport

import (
	"fmt"
	"math/rand/v2"
	"net"
	"testing"
)

// FreeLocalPort 查找指定范围内的空闲端口；调用方应立即启动监听，端口未被预留。
func FreeLocalPort(t *testing.T, minPort, maxPort int) int {
	t.Helper()
	count := maxPort - minPort + 1
	start := rand.IntN(count)
	for offset := range count {
		port := minPort + (start+offset)%count
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			if err := listener.Close(); err != nil {
				t.Fatal(err)
			}
			return port
		}
	}
	t.Fatal("no free port in the specified range")
	return 0
}
