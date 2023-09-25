package proxy

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/angrybayblade/tunnel/proxy/headers"
	"github.com/urfave/cli/v2"
)

type Connection struct {
	free bool
	conn net.Conn
}

func (c *Connection) Forward(requestHeader *headers.HttpRequestHeader, requestConn net.Conn) {
	// Investigate the usage of net.Pipe
	pumpBytes := make([]byte, 1024)
	requestHeader.Write(c.conn)
	if requestHeader.Headers["Content-Length"] != "" {
		for {
			n, err := requestConn.Read(pumpBytes)
			if err != nil {
				break
			}
			c.conn.Write(pumpBytes[:n])
			if string(pumpBytes[n-4:n]) == headers.HttpHeaderSeparator {
				break
			}
		}
	}
	for {
		n, err := c.conn.Read(pumpBytes)
		if err != nil {
			break
		}
		requestConn.Write(pumpBytes[:n])
	}
	requestConn.Close()
	c.conn.Close()
}

type Session struct {
	connections map[string]*Connection
	free        []string
	inUse       []string
	connected   int
}

func (s *Session) Join(id string, conn net.Conn) {
	s.connections[id] = &Connection{
		free: true,
		conn: conn,
	}
	s.free = append(s.free, id)
	s.connected += 1
}

func (s *Session) Forward(requestHeader *headers.HttpRequestHeader, rquestConn net.Conn) error {
	var err error
	if s.connected <= 0 {
		// TODO: Extract to method
		defer rquestConn.Close()
		response := [][]byte{
			[]byte("HTTP/1.1 200 OK\r\n"),
			[]byte("Date: Thu, 14 Sep 2023 12:28:53 GMT\r\n"),
			[]byte("Server: Go-Tunnel/0.1.0 (Ubuntu)\r\n"),
			[]byte("Content-Length: 53\r\n"),
			[]byte("Content-Type: text/html\r\n"),
			[]byte("Connection: Closed\r\n"),
			[]byte("\r\n"),
			[]byte("{\"error\": \"No free connection available in the pool\"}"),
		}
		for _, l := range response {
			_, err = rquestConn.Write(l)
			if err != nil {
				return fmt.Errorf("Request forward fail, no free connections available; Error writing response: %v", err)
			}
		}
		return ErrForwardFailedNoFreeConnection
	}
	freeConnectionIndex := s.free[0]
	s.free = s.free[1:]
	s.inUse = append(s.inUse, freeConnectionIndex)
	freeConnection := s.connections[freeConnectionIndex]
	freeConnection.Forward(requestHeader, rquestConn)
	delete(s.connections, freeConnectionIndex)
	s.connected -= 1
	return nil
}

type ForwardProxy struct {
	Addr            Addr
	Logger          *log.Logger
	Ln              net.Listener
	quitch          chan error
	sessions        map[string]*Session
	requestHandlers map[string]interface{}
}

func (fp *ForwardProxy) Start() error {
	Ln, err := net.Listen(
		"tcp", fp.Addr.ToString(),
	)
	if err != nil {
		return err
	}

	fp.Ln = Ln
	fp.sessions = make(map[string]*Session)
	fp.requestHandlers = map[string]interface{}{
		headers.RpRequestCreate: fp.handleCreate,
		headers.RpRequestJoin:   fp.handleJoin,
		headers.RpRequestDelete: fp.handleDelete,
	}
	fp.quitch = make(chan error)

	// Start listener
	defer Ln.Close()
	go fp.Listen()

	// Wait for the process to complete
	return <-fp.quitch
}

func (fp *ForwardProxy) Listen() {
	for {
		conn, err := fp.Ln.Accept()
		if err != nil {
			// fp.Logger.Printf("Erorr accepting the connection")
			continue
		}
		go fp.Handle(conn)
	}
}

func (fp *ForwardProxy) handleCreate(request *headers.ProxyHeader, conn net.Conn) {
	sessionKey := createSesssionKey([]byte(request.Key))
	responseHeader := headers.ProxyHeader{
		Code: headers.FpStatusSuccess,
		Key:  sessionKey,
	}
	responseHeader.Write(conn)
	fp.sessions[sessionKey] = &Session{
		connections: make(map[string]*Connection, 5),
		free:        make([]string, 0),
		inUse:       make([]string, 0),
	}
	conn.Close()
	fp.Logger.Println("/CREATE", sessionKey)
}

func (fp *ForwardProxy) handleJoin(request *headers.ProxyHeader, conn net.Conn) {
	if fp.sessions[request.Key].connected >= MAX_CONNECTION_POOL_SIZE {
		response := &headers.ProxyHeader{
			Code: headers.FpStatusMaxConnectionsLimitReached,
			Key:  request.Key,
		}
		_, err := response.Write(conn)
		if err != nil {
			fp.Logger.Println("/JOIN", request.Key, "-> Error writing response:", err.Error())
		} else {
			fp.Logger.Println("/JOIN", request.Key, "-> No free connection available")
		}
	} else {
		// request.Message represents connection id
		response := &headers.ProxyHeader{
			Code: headers.FpStatusSuccess,
			Key:  request.Key,
		}
		_, err := response.Write(conn)
		if err != nil {
			fp.Logger.Println("/JOIN", request.Key, "-> Error writing response:", err.Error())
		} else {
			fp.sessions[request.Key].Join(request.Message, conn)
			fp.Logger.Println("/JOIN", request.Key, "-> Connection ID:", request.Message)
		}
	}
}

func (fp *ForwardProxy) handleDelete(request *headers.ProxyHeader, conn net.Conn) {
}

func (fp *ForwardProxy) handleForward(request *headers.HttpRequestHeader, conn net.Conn) {
	var err error
	sessionKey := strings.Split(request.Headers["Host"], ".")[0]
	session := fp.sessions[sessionKey]
	if session == nil {
		response := [][]byte{
			[]byte("HTTP/2 200 OK\r\n"),
			[]byte("Date: Thu, 14 Sep 2023 12:28:53 GMT\r\n"),
			[]byte("Server: Go-Tunnel/0.1.0 (Ubuntu)\r\n"),
			[]byte("Content-Length: 53\r\n"),
			[]byte("Content-Type: text/html\r\n"),
			[]byte("Connection: Closed\r\n"),
			[]byte("\r\n"),
			[]byte("{\"error\": \"No connection available in the pool\"}"),
		}
		for _, l := range response {
			_, err = conn.Write(l)
			if err != nil {
				fp.Logger.Println("Error writing response ... :", err.Error())
				break
			}
		}
		conn.Close()
		fp.Logger.Println("/FORWARD", sessionKey, "-> No session found")
	} else {
		err = session.Forward(request, conn)
		if err != nil {
			fp.Logger.Println("/FORWARD", sessionKey, err.Error())
		} else {
			fp.Logger.Println("/FORWARD", sessionKey, request.Method, request.Path, request.Protocol)
		}
	}
}

func (fp *ForwardProxy) Handle(conn net.Conn) {
	headerBytes := make([]byte, 1)
	_, err := conn.Read(headerBytes)
	if err != nil {
		fp.Logger.Println("Error reading first request byte")
		return
	}
	requestHandler := fp.requestHandlers[string(headerBytes)]
	if requestHandler != nil {
		requestHeader := &headers.ProxyHeader{}
		requestHeader.ReadPartial(conn, headerBytes)
		requestHandler.(func(*headers.ProxyHeader, net.Conn))(requestHeader, conn)
	} else {
		requestHeader := &headers.HttpRequestHeader{
			Buffer: headerBytes,
		}
		err = requestHeader.Read(conn)
		if err != nil {
			fp.Logger.Println(err)
			return
		}
		fp.handleForward(requestHeader, conn)
	}
}

func (fp *ForwardProxy) Stop() {
	fp.Ln.Close()
}

func Listen(cCtx *cli.Context) error {
	var port int = cCtx.Int("port")
	var host string = cCtx.String("host")

	fmt.Printf("Starting listener @ %s:%d\n", host, port)
	listener := &ForwardProxy{
		Addr: Addr{
			host: host,
			port: port,
		},
		Logger: log.New(
			os.Stdout, "FP: ", log.Ltime,
		),
	}
	err := listener.Start()
	return err
}
