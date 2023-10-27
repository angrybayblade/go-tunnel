package cmd

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"

	"github.com/angrybayblade/tunnel/proxy"
	"github.com/angrybayblade/tunnel/proxy/headers"
	"github.com/urfave/cli/v2"
)

func revokeKey(cCtx *cli.Context) error {
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
	proxyDial, _ := proxy.ConnectTo(cCtx.String("proxy"), true, 80)
	request := &headers.ProxyHeader{
		Code: headers.ProxyRequestRevokeKey,
		Key:  string(requestEnc),
	}
	request.Write(proxyDial)
	response := &headers.ProxyHeader{}
	response.Read(proxyDial)
	if response.Code == headers.ProxyResponseNotInUimaMode {
		return errors.New("Proxy not running in the UIMA mode")
	}

	if response.Code == headers.ProxyResponseAuthError {
		return errors.New("Invalid signing key")
	}

	if response.Key == string(make([]byte, 43)) {
		return errors.New("Key was not found with ID: " + strconv.Itoa(int(keyId)))
	}

	fmt.Println("Deleted", "\n  ID:", keyId, "\n  Key:", response.Key)
	return nil
}

var RevokeKey *cli.Command = &cli.Command{
	Name:   "revoke-key",
	Usage:  "Revoke authentication key",
	Action: revokeKey,
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
}
