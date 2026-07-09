package utils

import (
	"net"
	"os"
	"strconv"
	"syscall"
	"time"

	gopsnet "github.com/shirou/gopsutil/v3/net"
	"github.com/radiospiel/simple-go/src/logger"
	"github.com/samber/lo"
)

func killAll(pids []int, sig syscall.Signal) []int {
	return lo.Filter(pids, func(pid int, _ int) bool {
		return syscall.Kill(pid, sig) == nil
	})
}


// KillListeners sends SIGTERM to all processes listening on the given address,
// waits up to 3 seconds, then sends SIGKILL to any that remain.
func KillListeners(addr string) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return
	}

	portNum, err := strconv.ParseUint(port, 10, 32)
	if err != nil {
		return
	}

	conns, err := gopsnet.Connections("all")
	if err != nil {
		return
	}

	seen := map[int]struct{}{}
	var pids []int
	for _, c := range conns {
		if c.Laddr.Port == uint32(portNum) && c.Pid != 0 {
			pid := int(c.Pid)
			if pid != os.Getpid() {
				if _, ok := seen[pid]; !ok {
					seen[pid] = struct{}{}
					pids = append(pids, pid)
				}
			}
		}
	}
	if len(pids) == 0 {
		return
	}

	logger.Warn("Killing %d process(es) listening on port %s", len(pids), port)
	killAll(pids, syscall.SIGTERM)

	deadline := time.After(3 * time.Second)
	for pids = killAll(pids, 0); len(pids) > 0; pids = killAll(pids, 0) {
		select {
		case <-deadline:
			killAll(pids, syscall.SIGKILL)
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
}
