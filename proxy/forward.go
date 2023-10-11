package proxy

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/angrybayblade/tunnel/auth"
	"github.com/angrybayblade/tunnel/proxy/headers"
)

type Connection struct {
	free bool
	conn net.Conn
}

func (c *Connection) Forward(requestHeader *headers.HttpRequestHeader, requestConn net.Conn) {
	var pumpBytes []byte
	requestHeader.Write(c.conn)
	if requestHeader.Headers["Content-Length"] != "" {
		contenLength, _ := strconv.Atoi(requestHeader.Headers["Content-Length"])
		if contenLength <= HttpRequestPipeChunkSize {
			pumpBytes = make([]byte, contenLength)
			requestConn.Read(pumpBytes)
			c.conn.Write(pumpBytes)
		} else {
			iter := int(math.Floor(float64(contenLength) / float64(HttpRequestPipeChunkSize)))
			pumpBytes = make([]byte, HttpRequestPipeChunkSize)
			for i := 0; i < iter; i++ {
				requestConn.Read(pumpBytes)
				c.conn.Write(pumpBytes)
			}
			pumpBytes = make([]byte, contenLength%HttpRequestPipeChunkSize)
			requestConn.Read(pumpBytes)
			c.conn.Write(pumpBytes)
		}
	}

	pumpBytes = make([]byte, HttpRequestPipeChunkSize)
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
	if s.connected <= 0 {
		defer rquestConn.Close()
		_, err := headers.HttpResponseNoFreeConnection.Write(rquestConn)
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
	Uima            bool
	sessions        map[string]*Session
	requestHandlers map[string]interface{}
	running         bool
	auth            auth.AuthSession
	mut             *sync.Mutex
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
		headers.RpRequestCreate:      fp.handleCreate,
		headers.RpRequestJoin:        fp.handleJoin,
		headers.RpRequestDelete:      fp.handleDelete,
		headers.FpRequestGenerateKey: fp.handleGenerateKey,
		headers.FpRequestRevokeKey:   fp.handleRevokeKey,
	}
	fp.running = true
	fp.mut = &sync.Mutex{}
	if fp.Uima {
		kp, err := auth.GenerateKeyPair()
		if err != nil {
			return err
		}
		fp.auth = auth.NewInMemorySession(kp)
		pk, _ := kp.DumpPublicKey()
		file := os.Getenv("PROXY_PUBLIC_KEY_FILE")
		if file == "" {
			file = "key.pub"
		}
		os.WriteFile(file, pk, 0644)
		fp.Logger.Println("Using in-memory authentication server; RSA public key for authentication is stored in", file)
	} else {
		fp.auth = auth.NewDefaultSession(DUMMY_KEY)
		fp.Logger.Println("Using default key authentication server")
	}
	return nil
}

func (fp *ForwardProxy) Runing() bool {
	fp.mut.Lock()
	running := fp.running
	fp.mut.Unlock()
	return running
}

func (fp *ForwardProxy) Listen() {
	for fp.Runing() {
		conn, err := fp.Ln.Accept()
		if err != nil {
			if !fp.Runing() {
				return
			}
			fp.Logger.Println("Erorr accepting the connection:", err.Error())
			continue
		}
		go fp.Handle(conn)
	}
}

func (fp *ForwardProxy) handleCreate(request *headers.ProxyHeader, conn net.Conn) {
	if !fp.auth.IsValidAuthToken(request.Key) {
		responseHeader := headers.ProxyHeader{
			Code: headers.FpStatusAuthError,
			Key:  "",
		}
		responseHeader.Write(conn)
		return
	}
	sessionKey := auth.Sha256([]byte(request.Key))
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
	if fp.sessions[request.Key].connected >= MaxConnectionPoolSize {
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

func (fp *ForwardProxy) handleGenerateKey(request *headers.ProxyHeader, conn net.Conn) {
	var response *headers.ProxyHeader
	defer conn.Close()

	if !fp.Uima {
		fp.Logger.Printf("/GENERATE not in UIMA mode")
		response = &headers.ProxyHeader{
			Code: headers.FpStatusErrorNotInUimaMode,
		}
		response.Write(conn)
		return
	}

	if !fp.auth.IsValidRequest([]byte(request.Key), headers.FpRequestGenerateKey) {
		fp.Logger.Printf("/GENERATE Invalid signing key")
		response = &headers.ProxyHeader{
			Code: headers.FpStatusAuthError,
		}
		response.Write(conn)
		return
	}

	// TOFIX: Increase header size to allow different communication type
	if len(fp.auth.Store()) > 255 {
		fp.Logger.Printf("/GENERATE Max token limite reached")
		response = &headers.ProxyHeader{
			Code: headers.FpStatusMaxConnectionsLimitReached,
		}
		response.Write(conn)
		return
	}

	response = &headers.ProxyHeader{
		Code:    headers.FpStatusSuccess,
		Key:     fp.auth.GenerateKey(),
		Message: strconv.Itoa(fp.auth.Count()),
	}
	response.Write(conn)
	fp.Logger.Println("/GENERATE Generate key with index:", fp.auth.Count())
}

func (fp *ForwardProxy) handleRevokeKey(request *headers.ProxyHeader, conn net.Conn) {
	var response *headers.ProxyHeader
	defer conn.Close()

	if !fp.Uima {
		fp.Logger.Printf("/REVOKE not in UIMA mode")
		response = &headers.ProxyHeader{
			Code: headers.FpStatusErrorNotInUimaMode,
		}
		response.Write(conn)
		return
	}

	keyIdbuf, err := fp.auth.(*auth.InMemory).KeyPair.Decrypt([]byte(request.Key))
	if err != nil {
		fp.Logger.Printf("/REVOKE Invalid signing key")
		response = &headers.ProxyHeader{
			Code: headers.FpStatusAuthError,
		}
		response.Write(conn)
		return
	}

	keyIdbuf = append(keyIdbuf, 0)
	keyId := int(binary.LittleEndian.Uint16(keyIdbuf))
	keyToDelete := ""
	for key, id := range fp.auth.Store() {
		if id == keyId {
			keyToDelete = key
			break
		}
	}

	if keyToDelete != "" {
		fp.auth.DeleteKey(keyToDelete)
		fp.Logger.Println("/REVOKE Revoked key with index:", keyId)
	} else {
		fp.Logger.Println("/REVOKE Key was not found:", keyId)
	}

	response = &headers.ProxyHeader{
		Code: headers.FpStatusSuccess,
		Key:  keyToDelete,
	}
	response.Write(conn)
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
	fp.mut.Lock()
	fp.running = false
	fp.mut.Unlock()

	for key, session := range fp.sessions {
		session.Disconnect()
		fp.Logger.Println("/DELETE", key)
	}
	fp.Logger.Println("Stopping the listener...")
	fp.Ln.Close()
}
