package eventstream

import (
	"encoding/json"
	"io"
	"net"
	"syscall"
	"time"

	"github.com/seventv/api/data/events"
	"go.uber.org/zap"

	"github.com/seventv/eventapi/internal/global"
	"github.com/seventv/eventapi/internal/util"
)

func (es *EventStream) Read(gctx global.Context) {
	conn := util.GetConn(es.r).(*net.TCPConn)
	heartbeat := time.NewTicker(time.Duration(es.heartbeatInterval) * time.Millisecond)

	liveness := time.NewTicker(time.Second * 1)

	defer func() {
		heartbeat.Stop()
		es.Destroy()
		liveness.Stop()
	}()

	if err := es.Greet(gctx); err != nil {
		return
	}

	es.SetReady()

	for {
		if err := checkConn(conn); err != nil {
			return
		}

		select {
		case <-es.OnClose():
			return
		case <-gctx.Done():
			es.SendClose(events.CloseCodeRestart, time.Second*5)
			return
		case <-heartbeat.C:
			if err := es.SendHeartbeat(); err != nil {
				return
			}
		case <-liveness.C: // Connection liveness check
		case s := <-es.evm.DispatchChannel():
			if s == nil { // channel closed
				return
			}

			var msg events.Message[events.DispatchPayload]

			err := json.Unmarshal([]byte(*s), &msg)
			if err != nil {
				zap.S().Errorw("dispatch unmarshal error", "error", err)
				continue
			}

			// Dispatch the event to the client
			es.handler.OnDispatch(gctx, msg)
		}
	}
}

func checkConn(conn net.Conn) error {
	var sysErr error = nil
	rc, err := conn.(syscall.Conn).SyscallConn()
	if err != nil {
		return err
	}
	err = rc.Read(func(fd uintptr) bool {
		var buf []byte = []byte{0}
		n, _, err := syscall.Recvfrom(int(fd), buf, syscall.MSG_PEEK|syscall.MSG_DONTWAIT)
		switch {
		case n == 0 && err == nil:
			sysErr = io.EOF
		case err == syscall.EAGAIN || err == syscall.EWOULDBLOCK:
			sysErr = nil
		default:
			sysErr = err
		}
		return true
	})
	if err != nil {
		return err
	}

	return sysErr
}
