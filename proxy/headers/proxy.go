package headers

import (
	"net"
)

// Proxy header
// _______________________________________
// | STATUS_CODE | SESSION_KEY | MESSAGE |
// ---------------------------------------
//
// STATUS_CODE : 1 byte
// SESSION_KEY : 43 bytes
// MESSAGE : 6 bytes
// HEADER_LENGHT : 50 bytes

const API_KEY_LEN int = 43
const STATUS_CODE_LEN int = 1
const SESSION_KEY_LEN int = 43
const MESSAGE_LEN int = 6
const STATUS_HEADER_LEN int = STATUS_CODE_LEN + SESSION_KEY_LEN + MESSAGE_LEN

// Forward proxy header helpers
const FP_STATUS_SUCCESS string = "0"
const FP_STATUS_AUTH_ERROR string = "1"
const FP_STATUS_MAX_CONNECTIONS_LIMIT_REACHED string = "2"

const RP_REQUEST_CREATE string = "0"
const RP_REQUEST_JOIN string = "1"
const RP_REQUEST_DELETE string = "2"

var RP_REQUESTS []string = []string{
	RP_REQUEST_CREATE,
	RP_REQUEST_JOIN,
	RP_REQUEST_DELETE,
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
	if len(header) < STATUS_HEADER_LEN {
		header += string(make([]byte, STATUS_HEADER_LEN-len(header)))
	}
	return []byte(header)
}

func (ph *ProxyHeader) Parse(header [STATUS_HEADER_LEN]byte) {
	ph.Code = string(header[0])
	ph.Key = string(header[STATUS_CODE_LEN : STATUS_CODE_LEN+SESSION_KEY_LEN])
	ph.Message = string(header[STATUS_CODE_LEN+SESSION_KEY_LEN:])
}

func (ph *ProxyHeader) Read(conn net.Conn) error {
	headerBytes := make([]byte, STATUS_HEADER_LEN)
	_, err := conn.Read(headerBytes)
	if err != nil {
		return err
	}
	ph.Parse([STATUS_HEADER_LEN]byte(headerBytes))
	return nil
}

func (ph *ProxyHeader) ReadPartial(conn net.Conn, initialBuffer []byte) error {
	headerBytes := make([]byte, STATUS_HEADER_LEN-len(initialBuffer))
	_, err := conn.Read(headerBytes)
	if err != nil {
		return err
	}
	ph.Parse([STATUS_HEADER_LEN]byte(append(initialBuffer, headerBytes...)))
	return nil
}

func (ph *ProxyHeader) Write(conn net.Conn) (int, error) {
	return conn.Write(ph.Build())
}
