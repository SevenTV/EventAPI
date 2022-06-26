package main

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"

	"github.com/bugsnag/panicwrap"
	"github.com/seventv/common/redis"
	"github.com/seventv/eventapi/internal/app"
	"github.com/seventv/eventapi/internal/configure"
	"github.com/seventv/eventapi/internal/global"
	"github.com/seventv/eventapi/internal/health"
	"github.com/seventv/eventapi/internal/instance"
	"github.com/seventv/eventapi/internal/monitoring"
	"github.com/sirupsen/logrus"
)

var (
	Version = "development"
	Unix    = ""
	Time    = "unknown"
	User    = "unknown"
)

func init() {
	debug.SetGCPercent(2000)
	if i, err := strconv.Atoi(Unix); err == nil {
		Time = time.Unix(int64(i), 0).Format(time.RFC3339)
	}
}

func main() {
	config := configure.New()

	exitStatus, err := panicwrap.BasicWrap(func(s string) {
		logrus.Error(s)
	})
	if err != nil {
		logrus.Error("failed to setup panic handler: ", err)
		os.Exit(2)
	}

	if exitStatus >= 0 {
		os.Exit(exitStatus)
	}

	if !config.NoHeader {
		logrus.Info("7TV EventAPI")
		logrus.Infof("Version: %s", Version)
		logrus.Infof("build.Time: %s", Time)
		logrus.Infof("build.User: %s", User)
	}

	logrus.Debug("MaxProcs: ", runtime.GOMAXPROCS(0))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	c, cancel := context.WithCancel(context.Background())

	gCtx := global.New(c, config)

	{
		ctx, cancel := context.WithTimeout(gCtx, time.Second*15)
		redisInst, err := redis.Setup(ctx, redis.SetupOptions{
			Username:   gCtx.Config().Redis.Username,
			Password:   gCtx.Config().Redis.Password,
			Database:   gCtx.Config().Redis.Database,
			Addresses:  gCtx.Config().Redis.Addresses,
			Sentinel:   gCtx.Config().Redis.Sentinel,
			MasterName: gCtx.Config().Redis.MasterName,
		})
		cancel()
		if err != nil {
			logrus.WithError(err).Fatal("failed to connect to redis")
		}

		gCtx.Inst().Redis = instance.WrapRedis(redisInst)
		gCtx.Inst().Monitoring = monitoring.NewPrometheus(gCtx)
	}

	dones := []<-chan struct{}{}

	if gCtx.Config().API.Enabled {
		_, done := app.New(gCtx)
		dones = append(dones, done)
	}
	if gCtx.Config().Health.Enabled {
		dones = append(dones, health.New(gCtx))
	}
	if gCtx.Config().Monitoring.Enabled {
		dones = append(dones, monitoring.New(gCtx))
	}

	logrus.Info("running")

	done := make(chan struct{})
	go func() {
		<-sig
		cancel()
		go func() {
			select {
			case <-time.After(time.Minute):
			case <-sig:
			}
			logrus.Fatal("force shutdown")
		}()

		logrus.Info("shutting down")

		for _, ch := range dones {
			<-ch
		}

		close(done)
	}()

	<-done

	logrus.Info("shutdown")
	os.Exit(0)
}
