package headers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
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

var DefaultHttpProtocolVersion string = "HTTP/1.1"

func readHeaderLine(conn net.Conn) ([]byte, error) {
	var readSize int = 0
	rBuffer := make([]byte, 1)
	lineBytes := make([]byte, 0)
	for {
		n, err := conn.Read(rBuffer)
		if err != nil {
			return lineBytes, ErrIncompleteHeaderLine
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
	if len(headerSplit) < 3 {
		return fmt.Errorf("%v; "+string(hreq.Buffer), ErrInvalidHeaderStart)
	}

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

type HttpResponseHeader struct {
	Protocol      string
	StatusCode    int
	StatusMessage string
	Headers       map[string]string
	Data          []byte
	Buffer        []byte
}

func (hres *HttpResponseHeader) SetData(data []byte) {
	hres.Data = data
	hres.Headers["Content-Length"] = strconv.Itoa(len(hres.Data))
	hres.Headers["Content-Type"] = "application/octet"
}

func (hres *HttpResponseHeader) Build() {
	var response string
	response = hres.Protocol + Whitespace + strconv.Itoa(hres.StatusCode) + Whitespace + hres.StatusMessage + HttpHeaderLineSeparator
	for k, v := range hres.Headers {
		response += k + HeaderSplit + v + HttpHeaderLineSeparator
	}
	response += HttpHeaderLineSeparator
	hres.Buffer = []byte(response)
	if hres.Data != nil {
		hres.Buffer = append(hres.Buffer, hres.Data...)
	}
}

func (hres *HttpResponseHeader) SetJson(data map[string]string) {
	// TODO: Add error handling
	hres.Data, _ = json.Marshal(data)
	hres.Headers["Content-Length"] = strconv.Itoa(len(hres.Data))
	hres.Headers["Content-Type"] = "application/json"
}

func (hres *HttpResponseHeader) Write(conn net.Conn) (int, error) {
	if hres.Buffer == nil {
		hres.Build()
	}
	return conn.Write(hres.Buffer)
}

func MakeHttpResponse(protocol string, code int, headers map[string]string, data []byte, json map[string]string, compileBuffer bool) HttpResponseHeader {
	response := HttpResponseHeader{
		Protocol:      protocol,
		StatusCode:    code,
		StatusMessage: http.StatusText(code),
		Headers:       headers,
	}
	if data != nil {
		response.SetData(data)
	}
	if len(json) > 0 {
		response.SetJson(json)
	}
	if compileBuffer {
		response.Build()
	}
	return response
}

var HttpResponseNoFreeConnection HttpResponseHeader = MakeHttpResponse(
	DefaultHttpProtocolVersion,
	http.StatusNotFound,
	map[string]string{
		"Server":     " Go-Tunnel/0.1.0",
		"Connection": " Closed",
	},
	nil,
	map[string]string{"error": "No free connection available in the pool"},
	true,
)

var HttpResponseNoSessionFound HttpResponseHeader = MakeHttpResponse(
	DefaultHttpProtocolVersion,
	http.StatusNotFound,
	map[string]string{
		"Server":     " Go-Tunnel/0.1.0",
		"Connection": " Closed",
	},
	nil,
	map[string]string{"error": "No session found"},
	true,
)

var HttpResponseCannotConnectToLocalserver HttpResponseHeader = MakeHttpResponse(
	DefaultHttpProtocolVersion,
	http.StatusNotFound,
	map[string]string{
		"Server":     " Go-Tunnel/0.1.0",
		"Connection": " Closed",
	},
	nil,
	map[string]string{"error": "Cannot connect to the local adress"},
	true,
)
