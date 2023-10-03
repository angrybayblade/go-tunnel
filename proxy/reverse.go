package proxy

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/angrybayblade/tunnel/proxy/headers"
	"github.com/urfave/cli/v2"
)

const DUMMY_KEY string = "0000000000000000000000000000000000000000000"

type ReverseProxy struct {
	Addr        Addr
	Logger      *log.Logger
	Proxy       string
	key         string
	Quitch      chan error
	sessionKey  string
	waitGroup   *sync.WaitGroup
	connections chan int
}

func (rp *ReverseProxy) ProxyURI() string {
	if strings.Contains(rp.Proxy, ":") {
		return rp.Proxy
	}
	return rp.Proxy + ":" + "80"
}

func (rp *ReverseProxy) Connect() error {
	conn, err := net.Dial("tcp", rp.ProxyURI())
	if err != nil {
		return fmt.Errorf("Failed connecting to the proxy: %w", err)
	}

	createRequest := &headers.ProxyHeader{
		Code: headers.RpRequestCreate,
		Key:  rp.key,
	}
	_, err = createRequest.Write(conn)
	if err != nil {
		return fmt.Errorf("Failed creating session: %w", err)
	}

	createResponse := &headers.ProxyHeader{}
	err = createResponse.Read(conn)
	if err != nil {
		return fmt.Errorf("Could not get the response from the proxy: %w", err)
	}

	if createResponse.Code == headers.FpStatusAuthError {
		return ErrProxyAuth
	}

	rp.sessionKey = createResponse.Key
	rp.Quitch = make(chan error)
	rp.connections = make(chan int, MAX_CONNECTION_POOL_SIZE)
	for id := 0; id < MAX_CONNECTION_POOL_SIZE; id++ {
		rp.connections <- id
	}
	return nil
}

func (rp *ReverseProxy) Listen() {
	var id int
	var joinRequest *headers.ProxyHeader
	var joinResponse *headers.ProxyHeader
	var ticker *time.Ticker = time.NewTicker(3 * time.Second)

	fmt.Println("Starting reverse proxy @", "http://"+rp.sessionKey+"."+rp.Proxy)
	for {
		id = <-rp.connections
		for {
			proxyDial, err := net.Dial("tcp", rp.ProxyURI())
			if err != nil {
				rp.Logger.Println("Failed connecting to the proxy:", err.Error())
				<-ticker.C
				continue
			}

			joinRequest = &headers.ProxyHeader{
				Code:    headers.RpRequestJoin,
				Key:     rp.sessionKey,
				Message: strconv.Itoa(id),
			}
			_, err = joinRequest.Write(proxyDial)
			if err != nil {
				rp.Logger.Println("Failed joining the proxy pool:", err.Error())
				<-ticker.C
				continue
			}

			joinResponse = &headers.ProxyHeader{}
			err = joinResponse.Read(proxyDial)
			if err != nil {
				rp.Logger.Println("Could not get the response from the proxy:", err.Error())
				<-ticker.C
				continue
			}

			if joinResponse.Code == headers.FpStatusMaxConnectionsLimitReached {
				rp.Logger.Println("Max connections limit reached")
				break
			}

			if joinResponse.Code == headers.FpStatusAuthError {
				rp.Quitch <- ErrProxyInvalidSessionKey
				return
			}

			// Wait until we get request
			pumpBytes := make([]byte, 1)
			proxyDial.Read(pumpBytes)
			go rp.Pump(proxyDial, pumpBytes, id)
			break
		}
	}
}

func (rp *ReverseProxy) Pump(proxyDial net.Conn, pumpBytes []byte, id int) {
	localDial, err := net.Dial("tcp", rp.Addr.ToString())
	if err != nil {
		rp.Logger.Println("Error connecting to local server:", err)
		headers.HttpResponseCannotConnectToLocalserver.Write(proxyDial)
		proxyDial.Close()
		return
	}

	requestHeader := headers.HttpRequestHeader{
		Buffer: pumpBytes,
	}
	requestHeader.Read(proxyDial)
	localDial.Write(requestHeader.Buffer)
	if requestHeader.Headers["Content-Size"] != "" {
		contentSize, err := strconv.Atoi(requestHeader.Headers["Content-Size"])
		if err != nil {
			rp.Logger.Println("/FORWARD Conection ID:", id, "->", "Error reading request payload")
		} else {
			pumpBytes = make([]byte, contentSize)
			for {
				n, err := proxyDial.Read(pumpBytes)
				if err != nil {
					break
				}
				localDial.Write(pumpBytes[:n])
				if string(pumpBytes[n-4:n]) == headers.HttpHeaderSeparator {
					break
				}
			}
		}
	}

	for {
		n, err := localDial.Read(pumpBytes)
		if err != nil {
			break
		}
		proxyDial.Write(pumpBytes[:n])
	}
	localDial.Close()
	proxyDial.Close()
	rp.connections <- id
	rp.Logger.Println("/FORWARD Connection:", id, "->", requestHeader.Path, requestHeader.Method, requestHeader.Protocol)
}

func (rp *ReverseProxy) Disconnect() {
	rp.Logger.Println("Disconnecting...")
	conn, err := net.Dial("tcp", rp.ProxyURI())
	if err != nil {
		// The proxy is not running
		return
	}
	deleteSessionRequest := &headers.ProxyHeader{
		Code: headers.RpRequestDelete,
		Key:  rp.sessionKey,
	}
	deleteSessionRequest.Write(conn)
}

func Forward(cCtx *cli.Context) error {
	var port int = cCtx.Int("port")
	var host string = cCtx.String("host")
	var key string = cCtx.String("key")
	var Proxy string = cCtx.String("proxy")

	logger, err := getLogger("RP", cCtx.String("log"))
	if err != nil {
		return err
	}

	quitCh := make(chan error)
	proxy := &ReverseProxy{
		Addr: Addr{
			host: host,
			port: port,
		},
		key:    key,
		Proxy:  Proxy,
		Logger: logger,
	}
	err = proxy.Connect()
	if err != nil {
		return err
	}

	go proxy.Listen()
	go waitForTerminationSignal(quitCh)
	go func(waitChannel chan error, quitChannel chan error) {
		quitCh <- <-quitChannel
	}(quitCh, proxy.Quitch)

	err = <-quitCh
	proxy.Disconnect()
	return err
}
