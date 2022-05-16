package client

import (
	"bufio"
	"sync"

	"github.com/SevenTV/Common/structures/v3"
	"github.com/SevenTV/Common/structures/v3/events"
	"github.com/SevenTV/EventAPI/src/global"
	websocket "github.com/fasthttp/websocket"
	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
)

type WebSocket struct {
	c                 *websocket.Conn
	seq               int64
	evm               EventMap
	writeMtx          sync.Mutex
	heartbeatInterval int64
	heartbeatCount    int64
}

func NewWebSocket(gctx global.Context, conn *websocket.Conn) Connection {
	hbi := gctx.Config().API.HeartbeatInterval
	if hbi == 0 {
		hbi = 45000
	}

	ws := WebSocket{
		conn,
		0,
		NewEventMap(),
		sync.Mutex{},
		hbi,
		0,
	}

	return &ws
}

func (w *WebSocket) Greet() error {
	w.writeMtx.Lock()
	defer w.writeMtx.Unlock()
	msg, err := events.NewMessage(events.OpcodeHello, events.HelloPayload{
		HeartbeatInterval: int64(w.heartbeatInterval),
	})
	if err != nil {
		return err
	}

	return w.c.WriteJSON(msg)
}

func (w *WebSocket) Heartbeat() error {
	w.writeMtx.Lock()
	defer w.writeMtx.Unlock()
	w.heartbeatCount++
	msg, err := events.NewMessage(events.OpcodeHeartbeat, events.HeartbeatPayload{
		Count: w.heartbeatCount,
	})
	if err != nil {
		return err
	}

	return w.c.WriteJSON(msg)
}

func (w *WebSocket) Dispatch(t events.EventType, data []byte) error {
	w.writeMtx.Lock()
	defer w.writeMtx.Unlock()
	w.seq++
	msg, err := events.NewMessage(events.OpcodeDispatch, events.DispatchPayload[structures.User]{
		Type: t,
		Body: events.ChangeMap[structures.User]{},
	})
	if err != nil {
		return err
	}

	return w.c.WriteJSON(msg)
}

func (w *WebSocket) Close(code events.CloseCode) {
	w.writeMtx.Lock()
	defer w.writeMtx.Unlock()

	err := w.c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(int(code), code.String()))
	err = multierror.Append(err, w.c.Close()).ErrorOrNil()
	if err != nil {
		zap.S().Errorw("failed to close connection")
	}
}

func (w *WebSocket) Events() EventMap {
	return w.evm
}

func (*WebSocket) Actor() *structures.User {
	// TODO: Return the actor here when authentication is implemented
	return nil
}

// SendError implements Connection
func (w *WebSocket) SendError(txt string, fields map[string]any) {
	w.writeMtx.Lock()
	defer w.writeMtx.Unlock()

	if fields == nil {
		fields = map[string]any{}
	}
	msg, err := events.NewMessage(events.OpcodeError, events.ErrorPayload{
		Message: txt,
		Fields:  fields,
	})
	if err != nil {
		zap.S().Errorw("failed to set up an error message", "error", err)
		return
	}
	if err := w.c.WriteJSON(msg); err != nil {
		zap.S().Errorw("failed to write an error message to the socket", "error", err)
	}
}

func (*WebSocket) SetWriter(w *bufio.Writer) {
	zap.S().Fatalw("called SetWriter() on a WebSocket connection")
}
