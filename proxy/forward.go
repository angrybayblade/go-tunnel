package proxy

import (
	"fmt"
	"net"
	"strings"

	"github.com/angrybayblade/tunnel/proxy/headers"
	"github.com/urfave/cli/v2"
)

type Connection struct {
	free bool
	conn net.Conn
}

func (c *Connection) Forward(requestHeader *headers.HttpRequestHeader, requestConn net.Conn) {
	pumpBytes := make([]byte, 1024)
	requestHeader.Write(c.conn)
	if requestHeader.Headers["Content-Length"] != "" {
		for {
			n, err := requestConn.Read(pumpBytes)
			if err != nil {
				break
			}
			c.conn.Write(pumpBytes[:n])
			if string(pumpBytes[n-4:n]) == headers.HTTP_HEADER_SEPARATOR {
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

func (s *Session) Forward(requestHeader *headers.HttpRequestHeader, rquestConn net.Conn) {
	if len(s.free) == 0 {
		// TODO: Extract to method
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
			rquestConn.Write(l)
		}
		rquestConn.Close()
	}
	freeConnectionIndex := s.free[0]
	s.free = s.free[1:]
	s.inUse = append(s.inUse, freeConnectionIndex)
	freeConnection := s.connections[freeConnectionIndex]
	freeConnection.Forward(requestHeader, rquestConn)
	delete(s.connections, freeConnectionIndex)
	s.connected -= 1
}

type ForwardProxy struct {
	Addr            Addr
	Quitch          chan struct{}
	ln              net.Listener
	sessions        map[string]*Session
	requestHandlers map[string]interface{}
}

func (fp *ForwardProxy) Start() error {
	ln, err := net.Listen(
		"tcp", fp.Addr.ToString(),
	)
	if err != nil {
		return err
	}
	defer ln.Close()
	fp.sessions = make(map[string]*Session)
	fp.requestHandlers = map[string]interface{}{
		headers.RP_REQUEST_CREATE: fp.handleCreate,
		headers.RP_REQUEST_JOIN:   fp.handleJoin,
		headers.RP_REQUEST_DELETE: fp.handleDelete,
	}
	fp.ln = ln
	go fp.Listen()

	// Replace with waitgroup
	<-fp.Quitch
	return nil
}

func (fp *ForwardProxy) Listen() {
	for {
		conn, err := fp.ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		go fp.Handle(conn)
	}
}

func (fp *ForwardProxy) handleCreate(request *headers.ProxyHeader, conn net.Conn) {
	sessionKey := createSesssionKey([]byte(request.Key))
	responseHeader := headers.ProxyHeader{
		Code: headers.FP_STATUS_SUCCESS,
		Key:  sessionKey,
	}
	responseHeader.Write(conn)
	fp.sessions[sessionKey] = &Session{
		connections: make(map[string]*Connection, 5),
		free:        make([]string, 0),
		inUse:       make([]string, 0),
	}
	conn.Close()
}

func (fp *ForwardProxy) handleJoin(request *headers.ProxyHeader, conn net.Conn) {
	fmt.Println("Join request from: " + request.Key + " with connection id: " + request.Message)
	if fp.sessions[request.Key].connected >= MAX_CONNECTION_POOL_SIZE {
		response := &headers.ProxyHeader{
			Code: headers.FP_STATUS_MAX_CONNECTIONS_LIMIT_REACHED,
			Key:  request.Key,
		}
		response.Write(conn)
	} else {
		// request.Message represents connection id
		response := &headers.ProxyHeader{
			Code: headers.FP_STATUS_SUCCESS,
			Key:  request.Key,
		}
		response.Write(conn)
		fp.sessions[request.Key].Join(request.Message, conn)
	}
}

func (fp *ForwardProxy) handleDelete(request *headers.ProxyHeader, conn net.Conn) {
}

func (fp *ForwardProxy) handleForward(request *headers.HttpRequestHeader, conn net.Conn) {
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
			conn.Write(l)
		}
		conn.Close()
		return
	}
	fmt.Println("Forwarding request for:", sessionKey)
	session.Forward(request, conn)
}

func (fp *ForwardProxy) Handle(conn net.Conn) {
	headerBytes := make([]byte, 1)
	_, err := conn.Read(headerBytes)
	if err != nil {
		fmt.Println("Error reading ", err)
	}
	requestHandler := fp.requestHandlers[string(headerBytes)]
	if requestHandler != nil {
		requestHeader := &headers.ProxyHeader{}
		requestHeader.ReadPartial(conn, headerBytes)
		requestHandler.(func(*headers.ProxyHeader, net.Conn))(requestHeader, conn)
	} else {
		requestHeader := &headers.HttpRequestHeader{}
		err = requestHeader.Read(conn, headerBytes)
		if err != nil {
			fmt.Println(err)
		}

		fp.handleForward(requestHeader, conn)
	}

}

func (fp *ForwardProxy) Stop() {
	fp.ln.Close()
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
		Quitch: make(chan struct{}),
	}
	err := listener.Start()
	if err != nil {
		panic(err)
	}
	return nil
}
