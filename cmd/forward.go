package cmd

import (
	"github.com/angrybayblade/tunnel/proxy"
	"github.com/urfave/cli/v2"
)

func forward(cCtx *cli.Context) error {
	var port int = cCtx.Int("port")
	var host string = cCtx.String("host")
	var key string = cCtx.String("key")
	var Proxy string = cCtx.String("proxy")

	logger, err := getLogger("RP", cCtx.String("log"))
	if err != nil {
		return err
	}

	quitCh := make(chan error)
	proxy := &proxy.ReverseProxy{
		Addr: proxy.Addr{
			Host: host,
			Port: port,
		},
		Key:    key,
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

var Forward *cli.Command = &cli.Command{
	Name:   "forward",
	Usage:  "Forward the port to the given proxy service",
	Action: forward,
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:  "port",
			Value: 3000,
			Usage: "Port to listen",
		},
		&cli.StringFlag{
			Name:  "host",
			Value: "127.0.0.1",
			Usage: "Host to serve",
		},
		&cli.StringFlag{
			Name:  "key",
			Value: proxy.DUMMY_KEY,
			Usage: "API key for proxy server",
		},
		&cli.StringFlag{
			Name:  "proxy",
			Value: "localhost:3000",
			Usage: "URI for proxy server",
		},
		&cli.StringFlag{
			Name:  "log",
			Usage: "Logfile",
		},
	},
}
