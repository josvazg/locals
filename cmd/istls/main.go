package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"time"
)

// IsHTTPS checks if a target address (e.g., "localhost:8443") is serving TLS.
func IsHTTPS(address string) bool {
	dialer := &net.Dialer{Timeout: 2 * time.Second}

	config := &tls.Config{InsecureSkipVerify: true}

	conn, err := tls.DialWithDialer(dialer, "tcp", address, config)
	if err != nil {
		return false
	}
	defer conn.Close()

	return conn.ConnectionState().HandshakeComplete
}

func main() {
	address := os.Args[1]
	fmt.Printf("%s is HTTPS? %v", address, IsHTTPS(address))
}
