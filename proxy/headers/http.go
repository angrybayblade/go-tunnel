package headers

import (
	"bytes"
	"net"
)

// Header separators
var HttpHeaderLineSeparatorBytes []byte = []byte("\r\n")
var HttpHeaderSeparatorBytes []byte = []byte("\r\n\r\n")

var HttpHeaderLineSeparator string = "\r\n"
var HttpHeaderSeparator string = "\r\n\r\n"

const HttpHeaderLineSeparatorLen int = 2
const HttpHeaderSeparatorLen int = 4

var WhitespaceBytes []byte = []byte(" ")
var HeaderSplitBytes []byte = []byte(": ")

var Whitespace string = " "
var HeaderSplit string = ": "

func readHeaderLine(conn net.Conn) ([]byte, error) {
	var readSize int = 0
	rBuffer := make([]byte, 1)
	lineBytes := make([]byte, 0)
	for {
		n, err := conn.Read(rBuffer)
		if err != nil {
			return lineBytes, IncompleteHeaderLine
		}
		lineBytes = append(lineBytes, rBuffer...)
		readSize += n
		idx := readSize - HttpHeaderLineSeparatorLen
		if idx < 0 {
			continue
		}
		if string(lineBytes[idx:]) == HttpHeaderLineSeparator {
			return lineBytes[:idx], nil
		}
	}
}

type HttpRequestHeader struct {
	Method   string
	Path     string
	Protocol string
	Headers  map[string]string
	Buffer   []byte
}

func (hreq *HttpRequestHeader) Build() []byte {
	var header string
	header = hreq.Method + Whitespace + hreq.Path + Whitespace + hreq.Protocol + HttpHeaderLineSeparator
	for k, v := range hreq.Headers {
		header += k + HeaderSplit + v + HttpHeaderLineSeparator
	}
	header += HttpHeaderLineSeparator
	return []byte(header)
}

func (hreq *HttpRequestHeader) Read(conn net.Conn) error {
	var err error
	var lineBytes []byte

	if hreq.Headers == nil {
		hreq.Headers = make(map[string]string)
	}

	if hreq.Buffer == nil {
		hreq.Buffer = make([]byte, 0)
	}

	lineBytes, err = readHeaderLine(conn)
	if err != nil {
		return err
	}
	hreq.Buffer = append(hreq.Buffer, lineBytes...)
	headerSplit := bytes.SplitN(hreq.Buffer, WhitespaceBytes, 3)
	hreq.Buffer = append(hreq.Buffer, HttpHeaderLineSeparatorBytes...)
	hreq.Method = string(headerSplit[0])
	hreq.Path = string(headerSplit[1])
	hreq.Protocol = string(headerSplit[2])
	for {
		lineBytes, err = readHeaderLine(conn)
		if err != nil {
			return err
		}
		headerSplit = bytes.SplitN(lineBytes, HeaderSplitBytes, 2)
		hreq.Buffer = append(hreq.Buffer, lineBytes...)
		hreq.Buffer = append(hreq.Buffer, HttpHeaderLineSeparatorBytes...)
		if len(headerSplit) <= 1 {
			break
		}
		hreq.Headers[string(headerSplit[0])] = string(headerSplit[1])
	}
	return err
}

func (hreq *HttpRequestHeader) Write(conn net.Conn) {
	if hreq.Buffer != nil {
		conn.Write(hreq.Buffer)
	} else {
		conn.Write([]byte(hreq.Method + Whitespace + hreq.Path + Whitespace + hreq.Protocol + HttpHeaderLineSeparator))
		for k, v := range hreq.Headers {
			conn.Write([]byte(k + HeaderSplit + v + HttpHeaderLineSeparator))
		}
		conn.Write([]byte(HttpHeaderLineSeparator))
	}
}

// func (hreq *HttpRequestHeader) ReadPartial(conn net.Conn, initialBuffer []byte) error {

// }

// type HttpResponseHeader struct {
// 	StatusCode    int
// 	StatusMessage string
// 	Headers       map[string]string
// }

// func (hres *HttpResponseHeader) Build() []byte {
// 	var header string
// 	header = strconv.Itoa(hres.StatusCode) + WHITESPACE + hres.StatusMessage + HttpHeaderLineSeparator
// 	for k, v := range hres.Headers {
// 		header += k + HEADER_SPLIT + v + HttpHeaderLineSeparator
// 	}
// 	return []byte(header)
// }

// func (hres *HttpResponseHeader) Parse(header [STATUS_HEADER_LEN]byte) {

// }

// func (hres *HttpResponseHeader) Read(conn net.Conn) error {

// }

// func (hres *HttpResponseHeader) ReadPartial(conn net.Conn, initialBuffer []byte) error {

// }

// func (hres *HttpResponseHeader) Write(conn net.Conn) (int, error) {
// 	return conn.Write(hres.Build())
// }
