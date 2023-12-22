package main

import (
	"fmt"
	"io"
	"net"
	"sync"
)

type TrafficCounter struct {
	sync.Mutex
	Counters map[string]uint64
}

var (
	// 使用两个结构分别存储输入和输出流量，避免锁的竞争
	inTraffic  TrafficCounter = TrafficCounter{Counters: make(map[string]uint64)}
	outTraffic TrafficCounter = TrafficCounter{Counters: make(map[string]uint64)}
	allowedIPs                = map[string]struct{}{"127.0.0.1": {}, "127.0.0.2": {}}
)

func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		fmt.Println("Listen() failed, err: ", err)
		return
	}
	defer func(listener net.Listener) {
		_ = listener.Close()
	}(listener)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept() failed, err: ", err)
			continue
		}

		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		if !isIPAllowed(ip) {
			fmt.Println("IP: ", ip, " is not allowed")
			_ = conn.Close()
			// TODO 对非白名单IP的连接，应该进行一些处理，比如记录日志
			continue
		}

		fmt.Println("IP: ", ip, " is allowed")
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)

	target, err := net.Dial("tcp", "127.0.0.1:80")
	if err != nil {
		fmt.Println("Dial() failed, err: ", err)
		return
	}
	defer func(target net.Conn) {
		_ = target.Close()
	}(target)

	// 使用 copyAndCount 的并发执行版本（同时处理流向）
	go copyAndCount(target, conn, &inTraffic)
	copyAndCount(conn, target, &outTraffic) // Block until this copy is done
}

func copyAndCount(dst io.Writer, src io.Reader, counter *TrafficCounter) {
	n, err := io.Copy(dst, src)
	if err != nil {
		fmt.Println("Copy failed, err:", err)
	}
	if n > 0 {
		updateTrafficStats(src, uint64(n), counter)
	}
}

func updateTrafficStats(src io.Reader, bytes uint64, counter *TrafficCounter) {
	// Assume that we are identifying the traffic by the Source IP
	srcAddr := src.(net.Conn).RemoteAddr().(*net.TCPAddr).IP.String()

	counter.Lock()
	counter.Counters[srcAddr] += bytes
	counter.Unlock()
}

func isIPAllowed(ip string) bool {
	_, ok := allowedIPs[ip]
	return ok
}
