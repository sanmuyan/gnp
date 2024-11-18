package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"gnp/cmd/client/cmd"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-sigs
		logrus.Warn("shutting down process...")
		cancel()
	}()
	cmd.Execute(ctx)
}
