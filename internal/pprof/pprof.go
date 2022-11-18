package pprof

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"

	"github.com/seventv/eventapi/internal/global"
	"go.uber.org/zap"
)

func New(gCtx global.Context) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		if err := http.ListenAndServe(gCtx.Config().PProf.Bind, nil); err != nil {
			zap.S().Fatalw("pprof failed to listen",
				"error", err,
			)
		}
	}()

	go func() {
		<-gCtx.Done()
		fmt.Println("done")
		close(done)
	}()

	return done
}
