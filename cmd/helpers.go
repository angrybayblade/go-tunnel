package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/angrybayblade/tunnel/auth"
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

func getLogger(name string, logFile string) (*log.Logger, error) {
	var logger *log.Logger = log.New(
		os.Stdout, name+": ", log.Ltime,
	)
	if logFile != "" {
		fp, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return logger, fmt.Errorf("Error opening file: %v", err)
		}
		logger.SetOutput(fp)
	}
	return logger, nil
}

func waitForTerminationSignal(waitChannel chan error) {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGTERM, syscall.SIGINT)
	<-signalChannel
	waitChannel <- ErrSigterm
}
