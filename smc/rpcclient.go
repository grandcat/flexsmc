package smc

import (
	"errors"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
)

const (
	defaultSocket = "unix:/tmp/grpc.sock"
)

// DialSocket connects to a gRPC server via a local socket.
func DialSocket(socket string) (*grpc.ClientConn, error) {
	if socket == "" {
		socket = defaultSocket
	}

	conn, err := grpc.Dial(socket, grpc.WithDialer(socketDialer), grpc.WithInsecure())
	return conn, err
}

func socketDialer(addr string, timeout time.Duration) (net.Conn, error) {
	const netSep = "unix"
	isUnixSock := strings.HasPrefix(addr, netSep)
	s := strings.Index(addr, ":")
	if !isUnixSock || s < 0 {
		return nil, errors.New("unknown network type")
	}
	return net.DialTimeout(addr[:s], addr[s+1:], timeout)
}
