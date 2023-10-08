package cmd

import (
	"errors"
	"fmt"

	"github.com/angrybayblade/tunnel/proxy"
	"github.com/angrybayblade/tunnel/proxy/headers"
	"github.com/urfave/cli/v2"
)

func generateKey(cCtx *cli.Context) error {
	kp, err := loadPublicKey(cCtx)
	if err != nil {
		return err
	}

	requestEnc, err := kp.Encrypt([]byte(headers.FpRequestGenerateKey))
	if err != nil {
		return err
	}

	proxyDial, _ := proxy.ConnectTo(cCtx.String("proxy"), true, 80)
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

var GenerateKey *cli.Command = &cli.Command{
	Name:   "generate-key",
	Usage:  "Generate authentication key",
	Action: generateKey,
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
}
