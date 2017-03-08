package smc

import "google.golang.org/grpc"

const (
	defaultSocket = "localhost:13131"
)

// DialSocket connects to a gRPC server via localhost.
func DialSocket(bindAddr string) (*grpc.ClientConn, error) {
	if bindAddr == "" {
		bindAddr = defaultSocket
	}

	conn, err := grpc.Dial(bindAddr, grpc.WithInsecure())
	return conn, err
}

// func socketDialer(addr string, timeout time.Duration) (net.Conn, error) {
// 	const netSep = "unix"
// 	isUnixSock := strings.HasPrefix(addr, netSep)
// 	s := strings.Index(addr, ":")
// 	if !isUnixSock || s < 0 {
// 		return nil, errors.New("unknown network type")
// 	}
// 	return net.DialTimeout(addr[:s], addr[s+1:], timeout)
// }
