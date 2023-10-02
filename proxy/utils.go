package proxy

import (
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
