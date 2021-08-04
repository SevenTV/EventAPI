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

	"github.com/mitchellh/panicwrap"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Catch panics - send alert to discord channel optionally
	exitStatus, err := panicwrap.BasicWrap(panicHandler)
	if err != nil {
		log.WithError(err).Fatal("panic handler failed")
	}
	if exitStatus >= 0 {
		os.Exit(exitStatus)
	}

	log.Info("starting")

	configCode := configure.Config.GetInt("exit_code")
	if configCode > 125 || configCode < 0 {
		log.WithField("requested_exit_code", configCode).Warn("invalid exit code specified in config using 0 as new exit code")
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
			log.Fatal("force shutting down")
		}()
		log.WithField("sig", sig).Info("stop issued")
		cancel()

		<-serverDone
		log.Infoln("server shutdown")

		start := time.Now().UnixNano()

		log.WithField("duration", float64(time.Now().UnixNano()-start)/10e5).Infof("shutdown")
		os.Exit(configCode)
	}()

	log.Info("started")

	select {}
}

func panicHandler(output string) {
	log.Errorf("PANIC OCCURED:\n\n%s\n", output)
	// Try to send a message to discord
	// discord.SendPanic(output)

	os.Exit(1)
}
