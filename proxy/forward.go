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
	key         string
	connections map[string]*Connection
	free        []string
	inUse       []string
	connected   int
	logger      *log.Logger
}

func (s *Session) Join(id string, conn net.Conn) {
	s.connections[id] = &Connection{
		free: true,
		conn: conn,
	}
	s.free = append(s.free, id)
	s.connected += 1
}

func (s *Session) Disconnect() {
	for id, connection := range s.connections {
		s.logger.Println("/DELETE", s.key, "-> Connection ID:", id)
		connection.conn.Close()
	}
}

func (s *Session) Forward(requestHeader *headers.HttpRequestHeader, rquestConn net.Conn) error {
	var err error
	if s.connected <= 0 {
		defer rquestConn.Close()
		_, err = headers.HttpResponseNoFreeConnection.Write(rquestConn)
		if err != nil {
			return fmt.Errorf("Request forward fail, no free connections available; Error writing response: %v", err)
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
	Quitch          chan error
	sessions        map[string]*Session
	requestHandlers map[string]interface{}
	running         bool
}

func (fp *ForwardProxy) Setup() error {
	Ln, err := net.Listen(
		"tcp", fp.Addr.ToString(),
	)
	if err != nil {
		return err
	}

	fp.Ln = Ln
	fp.Quitch = make(chan error)
	fp.sessions = make(map[string]*Session)
	fp.requestHandlers = map[string]interface{}{
		headers.RpRequestCreate: fp.handleCreate,
		headers.RpRequestJoin:   fp.handleJoin,
		headers.RpRequestDelete: fp.handleDelete,
	}
	fp.running = true
	return nil
}

func (fp *ForwardProxy) Listen() {
	for fp.running {
		conn, err := fp.Ln.Accept()
		if err != nil {
			if !fp.running {
				break
			}
			fp.Logger.Println("Erorr accepting the connection:", err.Error())
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
		key:         sessionKey,
		connections: make(map[string]*Connection, 5),
		free:        make([]string, 0),
		inUse:       make([]string, 0),
		logger:      fp.Logger,
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
	fp.sessions[request.Key].Disconnect()
	delete(fp.sessions, request.Key)
	fp.Logger.Println("/DELETE", request.Key)
}

func (fp *ForwardProxy) handleForward(request *headers.HttpRequestHeader, conn net.Conn) {
	var err error
	sessionKey := strings.Split(request.Headers["Host"], ".")[0]
	session := fp.sessions[sessionKey]
	if session == nil {
		defer conn.Close()
		_, err = headers.HttpResponseNoSessionFound.Write(conn)
		if err != nil {
			fp.Logger.Println("/FORWARD", sessionKey, "-> No session found; Error writing response: ", err.Error())
		} else {
			fp.Logger.Println("/FORWARD", sessionKey, "-> No session found")
		}
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
	fp.running = false
	for key, session := range fp.sessions {
		session.Disconnect()
		fp.Logger.Println("/DELETE", key)
	}
	fp.Logger.Println("Stopping the listener...")
	fp.Ln.Close()
}

func Listen(cCtx *cli.Context) error {
	var port int = cCtx.Int("port")
	var host string = cCtx.String("host")

	quitCh := make(chan error)
	fmt.Printf("Starting listener @ %s:%d\n", host, port)
	proxy := &ForwardProxy{
		Addr: Addr{
			host: host,
			port: port,
		},
		Logger: log.New(
			os.Stdout, "FP: ", log.Ltime,
		),
	}

	err := proxy.Setup()
	if err != nil {
		return err
	}

	go proxy.Listen()
	go waitForTerminationSignal(quitCh)
	go func(waitChannel chan error, quitChannel chan error) {
		quitCh <- <-quitChannel
	}(quitCh, proxy.Quitch)

	err = <-quitCh
	proxy.Stop()
	return err
}
