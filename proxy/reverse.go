package proxy

import (
	"fmt"
	"net"
	"strings"

	"github.com/urfave/cli/v2"
)

type ReverseProxy struct {
	addr        Addr
	ln          net.Listener
	quitch      chan struct{}
	connections map[string]net.Conn
}

func (rp *ReverseProxy) Start() error {
	ln, err := net.Listen(
		"tcp", rp.addr.ToString(),
	)
	if err != nil {
		return err
	}
	defer ln.Close()

	rp.connections = make(map[string]net.Conn)
	rp.ln = ln
	go rp.Listen()

	// Replace with waitgroup
	<-rp.quitch
	return nil
}

func (rp *ReverseProxy) Listen() {
	for {
		conn, err := rp.ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println("Received connection from", conn.RemoteAddr())
		go rp.Handle(conn)
	}
}

func (rp *ReverseProxy) Handle(conn net.Conn) {
	var lbytes []byte
	var lstring string
	var head_split []string

	headers := make(map[string]string)
	headerBytes := make([]byte, 0)
	lbytes, lstring = readHeaderLine(conn)
	head_split = strings.Split(lstring, " ")
	if head_split[0] == "CREATE" {
		conn.Write([]byte("0"))
		key := head_split[1][:4]
		rp.connections[key] = conn
	} else {
		var r_host string
		var header_split []string
		headerBytes = append(headerBytes, lbytes...)
		for {
			lbytes, lstring = readHeaderLine(conn)
			headerBytes = append(headerBytes, lbytes...)
			header_split = strings.SplitN(lstring, ": ", 2)

			// TODO: Investigate
			if len(header_split) == 1 {
				break
			}
			headers[header_split[0]] = header_split[1]
		}
		r_host = strings.Split(headers["Host"], ".")[0]
		fmt.Println("Forwarding request for", r_host)
		r_conn := rp.connections[r_host]
		r_conn.Write(headerBytes)
		pumpBytes := make([]byte, 1024)
		for {
			n, err := r_conn.Read(pumpBytes)
			if err != nil {
				conn.Close()
				r_conn.Close()
				return
			}
			conn.Write(pumpBytes[:n])
		}
	}
}

func (rp *ReverseProxy) Stop() {
	rp.ln.Close()
}

func Listen(cCtx *cli.Context) error {
	var port int = cCtx.Int("port")
	var host string = cCtx.String("host")

	fmt.Printf("Starting listener @ %s:%d\n", host, port)
	listener := &ReverseProxy{
		addr: Addr{
			host: host,
			port: port,
		},
		quitch: make(chan struct{}),
	}
	err := listener.Start()
	if err != nil {
		panic(err)
	}
	return nil
}
