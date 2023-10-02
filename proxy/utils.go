package proxy

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func waitForTerminationSignal(waitChannel chan error) {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGTERM, syscall.SIGINT)
	<-signalChannel
	waitChannel <- ErrSigterm
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
