package client

import (
	"context"
	"io"
	"net"
	"syscall"
	"time"

	"github.com/SevenTV/EventAPI/src/global"
)

func (es *EventStream) Read(gctx global.Context) {
	lctx, cancel := context.WithCancel(gctx)
	go func() {
		defer func() {
			cancel()
		}()
		select {
		case <-es.ctx.Done():
		case <-gctx.Done():
		case <-lctx.Done():
		}
	}()

	conn := es.ctx.Conn().(*net.TCPConn)
	heartbeat := time.NewTicker(time.Duration(es.heartbeatInterval) * time.Millisecond)
	defer func() {
		defer cancel()
		heartbeat.Stop()
	}()

	if err := es.Greet(); err != nil {
		return
	}
	for {
		if err := es.checkConn(conn); err != nil {
			return
		}
		select {
		case <-gctx.Done():
			return
		case <-lctx.Done():
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
