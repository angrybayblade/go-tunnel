package proxy

import (
	"fmt"
	"net"
	"os"

	"github.com/urfave/cli/v2"
)

type ForwardProxy struct {
	addr   Addr
	uri    string
	key    string
	quitch chan struct{}
}

// TODO: Make configurable
const KEY string = "qwertyuiopasdfghjklzxcvbnm"
const HEADER string = "CREATE qwertyuiopasdfghjklzxcvbnm\r\n\r\n"

func (fp *ForwardProxy) Connect() error {
	fmt.Println("Starting proxy server @", "http://"+KEY[:4]+"."+fp.uri)
	for {
		conn, err := net.Dial("tcp", fp.uri)
		if err != nil {
			fmt.Println(err)
			return nil
		}

		_, err = conn.Write([]byte(HEADER))
		if err != nil {
			fmt.Println("Failed connecting to the proxy:", err.Error())
			os.Exit(1)
		}

		reply := make([]byte, 1)
		_, err = conn.Read(reply)
		if err != nil {
			fmt.Println("Could not get the response from the proxy:", err.Error())
			os.Exit(1)
		}

		if string(reply) == "0" {
			l_conn, err := net.Dial("tcp", fp.addr.ToString())
			if err != nil {
				fmt.Println("Error connecting to local server:", err)
				return nil
			}
			pumpBytes := make([]byte, 1024)
			for {
				n, err := conn.Read(pumpBytes)
				if err != nil {
					break
				}
				l_conn.Write(pumpBytes[:n])
				if string(pumpBytes[n-4:n]) == HEADER_SEPARATOR {
					break
				}
			}
			for {
				n, err := l_conn.Read(pumpBytes)
				if err != nil {
					break
				}
				conn.Write(pumpBytes[:n])
			}
			l_conn.Close()
			conn.Close()
		} else {
			fmt.Println("Error connecting to proxy")
		}
	}
}

func (fp *ForwardProxy) Listen() {

}

func (fp *ForwardProxy) Forward(conn net.Conn) {

}

func Forward(cCtx *cli.Context) error {
	var port int = cCtx.Int("port")
	var host string = cCtx.String("host")
	var key string = cCtx.String("key")
	var uri string = cCtx.String("proxy")
	proxy := &ForwardProxy{
		addr: Addr{
			host: host,
			port: port,
		},
		key:    key,
		uri:    uri,
		quitch: make(chan struct{}),
	}
	err := proxy.Connect()
	if err != nil {
		panic(err)
	}
	return nil
}
