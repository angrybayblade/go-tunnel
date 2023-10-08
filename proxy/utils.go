package proxy

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

func ConnectTo(addr string, exitOnErr bool, defaultPort int) (net.Conn, error) {
	if defaultPort == 0 {
		defaultPort = 80
	}

	if !strings.Contains(addr, ":") {
		addr += ":" + strconv.Itoa(defaultPort)
	}

	conn, err := net.Dial("tcp", addr)
	if exitOnErr && err != nil {
		fmt.Println("Error connecting to", addr+";", err)
		os.Exit(1)
	}
	return conn, err
}
