package cmd

import (
	"fmt"

	"github.com/angrybayblade/tunnel/proxy"
	"github.com/urfave/cli/v2"
)

func listen(cCtx *cli.Context) error {
	var port int = cCtx.Int("port")
	var host string = cCtx.String("host")
	var uima bool = cCtx.Bool("uima") // Use in-memory authentication server

	logger, err := getLogger("FP", cCtx.String("log"))
	if err != nil {
		return err
	}

	quitCh := make(chan error)
	fmt.Printf("Starting listener @ %s:%d\n", host, port)
	proxy := &proxy.ForwardProxy{
		Addr: proxy.Addr{
			Host: host,
			Port: port,
		},
		Logger: logger,
		Uima:   uima,
	}

	err = proxy.Setup()
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

var Listen *cli.Command = &cli.Command{
	Name:   "listen",
	Usage:  "Listen on a port for new forward requests",
	Action: listen,
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
			Name:  "log",
			Usage: "Logfile",
		},
		&cli.BoolFlag{
			Name:  "uima",
			Usage: "Use in-memory authentication server",
		},
	},
}
