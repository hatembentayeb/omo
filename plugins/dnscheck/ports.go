package dnscheck

import (
	"net"
	"strconv"
	"sync"
	"time"
)

func checkPorts(host string) []portResult {
	out := make([]portResult, len(commonPorts))
	var wg sync.WaitGroup
	for i, p := range commonPorts {
		i, p := i, p
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = dialPort(host, p.Port, p.Name)
		}()
	}
	wg.Wait()
	return out
}

func dialPort(host string, port int, name string) portResult {
	res := portResult{Port: port, Name: name}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	res.Latency = time.Since(start)
	if err != nil {
		res.Open = false
		res.Error = err.Error()
		return res
	}
	_ = conn.Close()
	res.Open = true
	return res
}
