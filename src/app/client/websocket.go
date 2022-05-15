package client

import (
	"sync"

	"github.com/SevenTV/Common/structures/v3/events"
	"github.com/SevenTV/EventAPI/src/global"
	websocket "github.com/fasthttp/websocket"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
)

type WebSocket struct {
	c        *websocket.Conn
	seq      int64
	writeMtx sync.Mutex
}

func NewWebSocket(gctx global.Context, conn *websocket.Conn) Connection {
	ws := WebSocket{
		conn,
		0,
		sync.Mutex{},
	}

	return &ws
}

func (w *WebSocket) Greet() error {
	w.writeMtx.Lock()
	defer w.writeMtx.Unlock()
	msg, err := events.NewMessage(events.OpcodeHello, events.HelloPayload{
		HeartbeatInterval: 60 * 1000,
	})
	if err != nil {
		return err
	}

	return w.c.WriteJSON(msg)
}

func (w *WebSocket) Dispatch(t events.MessageType, data []byte) error {
	w.writeMtx.Lock()
	defer w.writeMtx.Unlock()
	w.seq++
	msg, err := events.NewMessage(events.OpcodeDispatch, events.DispatchPayload{
		Type: t,
		Body: events.ChangeMap[events.EmptyObject]{},
	})
	if err != nil {
		return err
	}

	return w.c.WriteJSON(msg)
}

func (w *WebSocket) Close(code events.CloseCode) {
	err := w.c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(int(code), code.String()))
	err = multierror.Append(err, w.c.Close()).ErrorOrNil()
	if err != nil {
		zap.S().Errorw("failed to close connection")
	}
}
