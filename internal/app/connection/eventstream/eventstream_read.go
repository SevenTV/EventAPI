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

	dispatch := es.Digest().Dispatch.Subscribe(es.ctx, es.sessionID, 128)
	ack := es.Digest().Ack.Subscribe(es.ctx, es.sessionID, 5)

	defer func() {
		heartbeat.Stop()
		es.Destory()
	}()

	if err := es.Greet(); err != nil {
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
		case msg := <-dispatch:
			_ = es.handler.OnDispatch(gctx, msg)

		// Listen for acks (i.e in response to a session mutation)
		case msg := <-ack:
			if err := es.Write(msg.ToRaw()); err != nil {
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
