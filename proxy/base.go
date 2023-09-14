package proxy

import (
	"net"
	"strconv"
)

const HEADER_LINE_SEPARATOR string = "\r\n"
const HEADER_SEPARATOR string = "\r\n\r\n"

var HEADER_LINE_SEPARATOR_BYTES []byte = []byte(HEADER_LINE_SEPARATOR)
var HEADER_SEPARATOR_BYTES []byte = []byte(HEADER_SEPARATOR)

func readHeaderLine(conn net.Conn) ([]byte, string) {
	headerBytes := make([]byte, 0)
	rBuffer := make([]byte, 1)
	var parsed string
	for {
		_, err := conn.Read(rBuffer)
		if err != nil {
			continue
		}
		headerBytes = append(headerBytes, rBuffer...)
		parsed = string(headerBytes)
		idx := len(parsed) - 2
		if idx < 0 {
			continue
		}
		if parsed[idx:] == HEADER_LINE_SEPARATOR {
			return headerBytes, parsed[:idx]
		}
	}
}

type Addr struct {
	host string
	port int
}

func (a *Addr) ToString() string {
	return a.host + ":" + strconv.Itoa(a.port)
}
