package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SevenTV/EventAPI/src/configure"
	"github.com/SevenTV/EventAPI/src/redis"
	"github.com/SevenTV/EventAPI/src/server"
	"github.com/sirupsen/logrus"

	"github.com/mitchellh/panicwrap"
)

func main() {
	// Catch panics - send alert to discord channel optionally
	exitStatus, err := panicwrap.BasicWrap(panicHandler)
	if err != nil {
		logrus.WithError(err).Fatal("panic handler failed")
	}
	if exitStatus >= 0 {
		os.Exit(exitStatus)
	}

	logrus.Info("starting")

	configCode := configure.Config.GetInt("exit_code")
	if configCode > 125 || configCode < 0 {
		logrus.WithField("requested_exit_code", configCode).Warn("invalid exit code specified in config using 0 as new exit code")
		configCode = 0
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	redis.Init(configure.Config.GetString("redis_uri"))

	ctx, cancel := context.WithCancel(context.Background())

	_, serverDone := server.New(ctx, configure.Config.GetString("conn_type"), configure.Config.GetString("conn_uri"))

	go func() {
		sig := <-c
		go func() {
			select {
			case <-c:
			case <-time.After(time.Minute):
			}
			logrus.Fatal("force shutting down")
		}()
		logrus.WithField("sig", sig).Info("stop issued")
		cancel()

		<-serverDone
		logrus.Infoln("server shutdown")

		start := time.Now().UnixNano()

		logrus.WithField("duration", float64(time.Now().UnixNano()-start)/10e5).Infof("shutdown")
		os.Exit(configCode)
	}()

	logrus.Info("started")

	select {}
}

func panicHandler(output string) {
	logrus.Errorf("PANIC OCCURED:\n\n%s\n", output)
	// Try to send a message to discord
	// discord.SendPanic(output)

	os.Exit(1)
}
