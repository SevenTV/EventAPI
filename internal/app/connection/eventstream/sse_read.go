package eventstream

import (
	"io"
	"net"
	"syscall"
	"time"

	"github.com/seventv/api/data/events"
	"github.com/seventv/eventapi/internal/global"
	"go.uber.org/zap"
)

func (es *EventStream) Read(gctx global.Context) {
	conn := es.c.Conn().(*net.TCPConn)
	heartbeat := time.NewTicker(time.Duration(es.heartbeatInterval) * time.Millisecond)
	dispatch := make(chan events.Message[events.DispatchPayload])
	ack := make(chan events.Message[events.AckPayload])
	go es.Digest().Dispatch.Subscribe(es.ctx, es.sessionID, dispatch)
	go es.Digest().Ack.Subscribe(es.ctx, es.sessionID, ack)

	defer func() {
		heartbeat.Stop()
		es.cancel()
		close(dispatch)
		es.Close(events.CloseCodeRestart)
	}()

	if err := es.Greet(); err != nil {
		return
	}

	for {
		if err := checkConn(conn); err != nil {
			return
		}
		select {
		case <-es.c.Done():
			return
		case <-gctx.Done():
			return
		case <-es.ctx.Done():
			return
		case <-heartbeat.C:
			if err := es.Heartbeat(); err != nil {
				return
			}

		case msg := <-dispatch:
			// Filter by subscribed event types
			ev, ok := es.Events().Get(msg.Data.Type)
			if !ok {
				continue // skip if not subscribed to this
			}

			if !ev.Match(msg.Data.Condition) {
				continue
			}

			msg.Data.Condition = nil

			if err := es.write(msg.ToRaw()); err != nil {
				zap.S().Errorw("failed to write dispatch to connection",
					"error", err,
				)
				continue
			}
		// Listen for acks (i.e in response to a session mutation)
		case msg := <-ack:
			if err := es.write(msg.ToRaw()); err != nil {
				zap.S().Errorw("failed to write ack to connection",
					"error", err,
				)
				continue
			}
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
