package freeport

import "net"

// GetFreePort returns a free port to be used by sshd
func GetFreePort() (int, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return -1, err
	}
	defer func() {
		_ = l.Close()
	}()
	port := l.Addr().(*net.TCPAddr).Port
	return port, nil
}
