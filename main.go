package main

import (
	"log"
	"os"

	"github.com/angrybayblade/tunnel/proxy"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:   "listen",
				Usage:  "Listen on a port for new forward requests",
				Action: proxy.Listen,
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
				},
			},
			{
				Name:   "forward",
				Usage:  "Forward the port to the given listner service",
				Action: proxy.Forward,
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
				},
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
