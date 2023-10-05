package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/angrybayblade/tunnel/auth"
	"github.com/angrybayblade/tunnel/proxy"
	"github.com/angrybayblade/tunnel/proxy/headers"
	"github.com/urfave/cli/v2"
)

func loadPublicKey(cCtx *cli.Context) (*auth.KeyPair, error) {
	var file string = cCtx.Path("key")
	if file == "" {
		return nil, errors.New("Please provide key file path")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	kp := &auth.KeyPair{}
	err = kp.LoadPublicKey(data)
	if err != nil {
		return nil, err
	}
	return kp, nil
}

func GenerateKey(cCtx *cli.Context) error {
	kp, err := loadPublicKey(cCtx)
	if err != nil {
		return err
	}

	requestEnc, err := kp.Encrypt([]byte(headers.FpRequestGenerateKey))
	if err != nil {
		return err
	}

	proxyDial, err := net.Dial("tcp", cCtx.String("proxy"))
	if err != nil {
		return err
	}

	request := &headers.ProxyHeader{
		Code: headers.FpRequestGenerateKey,
		Key:  string(requestEnc),
	}
	request.Write(proxyDial)
	response := &headers.ProxyHeader{}
	response.Read(proxyDial)

	if response.Code == headers.FpStatusErrorNotInUimaMode {
		return errors.New("Proxy not running in the UIMA mode")
	}

	if response.Code == headers.FpStatusAuthError {
		return errors.New("Invalid signing key")
	}

	// TOFIX: Increase header size to allow different communication type
	if response.Code == headers.FpStatusMaxConnectionsLimitReached {
		return errors.New("Invalid signing key")
	}

	fmt.Println("Generated", "\n  ID:", response.Message, "\n  Key:", response.Key)
	return nil
}

func RevokeKey(cCtx *cli.Context) error {
	var keyId uint16 = uint16(cCtx.Int("id"))
	if keyId == 0 {
		return errors.New("Auth token ID cannot be 0")
	}

	kp, err := loadPublicKey(cCtx)
	if err != nil {
		return err
	}
	messageBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(messageBytes, keyId)
	requestEnc, err := kp.Encrypt(messageBytes[:1])
	proxyDial, err := net.Dial("tcp", cCtx.String("proxy"))
	if err != nil {
		return err
	}

	request := &headers.ProxyHeader{
		Code: headers.FpRequestRevokeKey,
		Key:  string(requestEnc),
	}
	request.Write(proxyDial)
	response := &headers.ProxyHeader{}
	response.Read(proxyDial)
	if response.Code == headers.FpStatusErrorNotInUimaMode {
		return errors.New("Proxy not running in the UIMA mode")
	}

	if response.Code == headers.FpStatusAuthError {
		return errors.New("Invalid signing key")
	}

	if response.Key == string(make([]byte, 43)) {
		return errors.New("Key was not found with ID: " + strconv.Itoa(int(keyId)))
	}

	fmt.Println("Deleted", "\n  ID:", keyId, "\n  Key:", response.Key)
	return nil
}

func main() {
	app := &cli.App{
		Name:  "tunnel",
		Usage: "A naive HTTP tunnel implementation",
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
					&cli.StringFlag{
						Name:  "log",
						Usage: "Logfile",
					},
					&cli.BoolFlag{
						Name:  "uima",
						Usage: "Use in-memory authentication server",
					},
				},
			},
			{
				Name:   "forward",
				Usage:  "Forward the port to the given proxy service",
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
					&cli.StringFlag{
						Name:  "log",
						Usage: "Logfile",
					},
				},
			},
			{
				Name:   "generate-key",
				Usage:  "Generate authentication key",
				Action: GenerateKey,
				Flags: []cli.Flag{
					&cli.PathFlag{
						Name:  "key",
						Usage: "Key for signing the request",
					},
					&cli.StringFlag{
						Name:  "proxy",
						Value: "localhost:3000",
						Usage: "URI for proxy server",
					},
				},
			},
			{
				Name:   "revoke-key",
				Usage:  "Revoke authentication key",
				Action: RevokeKey,
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:  "id",
						Usage: "ID of the auth token",
					},
					&cli.StringFlag{
						Name:  "key",
						Usage: "Key for signing the request",
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
		if err != proxy.ErrSigterm {
			fmt.Println(err)
		}
		os.Exit(1)
	}
}
