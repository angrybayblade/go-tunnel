package headers

import (
	"net"
)

// Proxy header
// ______________________________________________________
// | STATUS_CODE / REQUEST_CODE | SESSION_KEY | MESSAGE |
// ------------------------------------------------------
//
// STATUS_CODE : 1 byte
// REQUEST_CODE : 1 byte
// SESSION_KEY : 43 bytes
// MESSAGE : 6 bytes
// HEADER_LENGHT : 50 bytes

const ApiKeyLen int = 43
const StatusCodeLen int = 1
const SessionKeyLen int = 43
const MessageLen int = 6
const StatusHeaderLen int = StatusCodeLen + SessionKeyLen + MessageLen

// Forward proxy header helpers
const FpStatusSuccess string = "0"
const FpStatusAuthError string = "1"
const FpStatusMaxConnectionsLimitReached string = "2"

const RpRequestCreate string = "0"
const RpRequestJoin string = "1"
const RpRequestDelete string = "2"

var RP_REQUESTS []string = []string{
	RpRequestCreate,
	RpRequestJoin,
	RpRequestDelete,
}

type ProxyHeader struct {
	Code    string
	Key     string
	Message string
}

func (ph *ProxyHeader) Build() []byte {
	var header string
	header += ph.Code
	header += ph.Key
	header += ph.Message
	if len(header) < StatusHeaderLen {
		header += string(make([]byte, StatusHeaderLen-len(header)))
	}
	return []byte(header)
}

func (ph *ProxyHeader) Parse(header [StatusHeaderLen]byte) {
	ph.Code = string(header[0])
	ph.Key = string(header[StatusCodeLen : StatusCodeLen+SessionKeyLen])
	ph.Message = string(header[StatusCodeLen+SessionKeyLen:])
}

func (ph *ProxyHeader) Read(conn net.Conn) error {
	headerBytes := make([]byte, StatusHeaderLen)
	_, err := conn.Read(headerBytes)
	if err != nil {
		return err
	}
	ph.Parse([StatusHeaderLen]byte(headerBytes))
	return nil
}

func (ph *ProxyHeader) ReadPartial(conn net.Conn, initialBuffer []byte) error {
	headerBytes := make([]byte, StatusHeaderLen-len(initialBuffer))
	_, err := conn.Read(headerBytes)
	if err != nil {
		return err
	}
	ph.Parse([StatusHeaderLen]byte(append(initialBuffer, headerBytes...)))
	return nil
}

func (ph *ProxyHeader) Write(conn net.Conn) (int, error) {
	return conn.Write(ph.Build())
}
