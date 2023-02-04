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
	"github.com/seventv/eventapi/internal/pprof"
	"go.uber.org/zap"
)

var (
	Version = "development"
	Unix    = ""
	Time    = "unknown"
	User    = "unknown"
)

func init() {
	if i, err := strconv.Atoi(Unix); err == nil {
		Time = time.Unix(int64(i), 0).Format(time.RFC3339)
	}

	debug.SetMemoryLimit(1.75 * 1024 * 1024 * 1024) // 1.75GB
}

func memory() {
	// Force freeing memory
	debug.FreeOSMemory()

	time.AfterFunc(time.Second, memory)
}

func main() {
	config := configure.New()
	memory()

	exitStatus, err := panicwrap.BasicWrap(func(s string) {
		zap.S().Error(s)
	})
	if err != nil {
		zap.S().Errorf("failed to setup panic handler: ", err)
		os.Exit(2)
	}

	if exitStatus >= 0 {
		os.Exit(exitStatus)
	}

	if !config.NoHeader {
		zap.S().Info("7TV EventAPI")
		zap.S().Infof("Version: %s", Version)
		zap.S().Infof("build.Time: %s", Time)
		zap.S().Infof("build.User: %s", User)
	}

	zap.S().Debugf("MaxProcs: ", runtime.GOMAXPROCS(0))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGHUP, syscall.SIGILL, syscall.SIGTERM, syscall.SIGQUIT)

	c, cancel := context.WithCancel(context.Background())

	gctx := global.New(c, config)

	{
		ctx, cancel := context.WithTimeout(gctx, time.Second*15)
		redisInst, err := redis.Setup(ctx, redis.SetupOptions{
			Username:   gctx.Config().Redis.Username,
			Password:   gctx.Config().Redis.Password,
			Database:   gctx.Config().Redis.Database,
			Addresses:  gctx.Config().Redis.Addresses,
			Sentinel:   gctx.Config().Redis.Sentinel,
			MasterName: gctx.Config().Redis.MasterName,
		})
		cancel()
		if err != nil {
			zap.S().Fatalw("failed to connect to redis", "error", err)
		}

		gctx.Inst().Redis = instance.WrapRedis(redisInst)
		gctx.Inst().Monitoring = monitoring.NewPrometheus(gctx)
	}

	dones := []<-chan struct{}{}

	if gctx.Config().API.Enabled {
		_, done := app.New(gctx)
		dones = append(dones, done)
	}
	if gctx.Config().PProf.Enabled {
		done := pprof.New(gctx)
		dones = append(dones, done)
	}
	if gctx.Config().Health.Enabled {
		dones = append(dones, health.New(gctx))
	}
	if gctx.Config().Monitoring.Enabled {
		dones = append(dones, monitoring.New(gctx))
	}

	zap.S().Infof("running")

	done := make(chan struct{})
	go func() {
		s := <-sig
		cancel()

		go func() {
			select {
			case <-time.After(time.Minute):
			case <-sig:
			}
			zap.S().Fatal("force shutdown")
		}()

		zap.S().Infof("gracefully shutting down... (%s)", s.String())

		for _, ch := range dones {
			<-ch
		}

		close(done)
	}()

	<-done

	zap.S().Info("shutdown")
	os.Exit(0)
}
