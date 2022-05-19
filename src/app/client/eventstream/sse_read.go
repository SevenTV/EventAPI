package eventstream

import (
	"io"
	"net"
	"syscall"
	"time"

	"github.com/SevenTV/Common/events"
	"github.com/SevenTV/EventAPI/src/global"
)

func (es *EventStream) Read(gctx global.Context) {
	conn := es.c.Conn().(*net.TCPConn)
	heartbeat := time.NewTicker(time.Duration(es.heartbeatInterval) * time.Millisecond)
	dispatch := make(chan events.Message[events.DispatchPayload])
	go func() {
		es.Digest().Dispatch.Subscribe(es.ctx, es.sessionID, dispatch)
	}()

	go func() {
		defer func() {
			es.cancel()
			close(dispatch)
			heartbeat.Stop()
		}()
		select {
		case <-es.c.Done():
		case <-gctx.Done():
		case <-es.ctx.Done():
		}
	}()

	if err := es.Greet(); err != nil {
		return
	}

	for {
		if err := es.checkConn(conn); err != nil {
			return
		}
		select {
		case <-es.ctx.Done():
			return
		case <-es.c.Done():
			return
		case <-heartbeat.C:
			if err := es.Heartbeat(); err != nil {
				return
			}
		}
	}
}

func (es *EventStream) checkConn(conn net.Conn) error {
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
