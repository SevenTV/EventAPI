package websocket

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"sync"

	websocket "github.com/fasthttp/websocket"
	"github.com/hashicorp/go-multierror"
	"github.com/seventv/api/data/events"
	"github.com/seventv/common/structures/v3"
	client "github.com/seventv/eventapi/internal/app/connection"
	"github.com/seventv/eventapi/internal/global"
	"go.uber.org/zap"
)

type WebSocket struct {
	c                 *websocket.Conn
	ctx               context.Context
	cancel            context.CancelFunc
	seq               int64
	evm               client.EventMap
	dig               client.EventDigest
	writeMtx          *sync.Mutex
	sessionID         []byte
	heartbeatInterval int64
	heartbeatCount    int64
}

func NewWebSocket(gctx global.Context, conn *websocket.Conn, dig client.EventDigest) (client.Connection, error) {
	hbi := gctx.Config().API.HeartbeatInterval
	if hbi == 0 {
		hbi = 45000
	}

	sessionID, err := client.GenerateSessionID(32)
	if err != nil {
		return nil, err
	}

	lctx, cancel := context.WithCancel(context.Background())
	ws := WebSocket{
		c:                 conn,
		ctx:               lctx,
		cancel:            cancel,
		seq:               0,
		evm:               client.NewEventMap(make(chan string, 10)),
		dig:               dig,
		writeMtx:          &sync.Mutex{},
		sessionID:         sessionID,
		heartbeatInterval: hbi,
		heartbeatCount:    0,
	}

	return &ws, nil
}

func (w *WebSocket) Context() context.Context {
	return w.ctx
}

func (w *WebSocket) SessionID() string {
	return hex.EncodeToString(w.sessionID)
}

func (w *WebSocket) Greet() error {
	w.writeMtx.Lock()
	defer w.writeMtx.Unlock()
	msg := events.NewMessage(events.OpcodeHello, events.HelloPayload{
		HeartbeatInterval: int64(w.heartbeatInterval),
		SessionID:         hex.EncodeToString(w.sessionID),
	})

	return w.c.WriteJSON(msg)
}

func (w *WebSocket) Heartbeat() error {
	w.writeMtx.Lock()
	defer w.writeMtx.Unlock()
	w.heartbeatCount++
	msg := events.NewMessage(events.OpcodeHeartbeat, events.HeartbeatPayload{
		Count: w.heartbeatCount,
	})

	return w.c.WriteJSON(msg)
}

func (w *WebSocket) Close(code events.CloseCode) {
	// Send "end of stream" message
	msg := events.NewMessage(events.OpcodeEndOfStream, events.EndOfStreamPayload{
		Code:    code,
		Message: code.String(),
	})

	if err := w.write(msg.ToRaw()); err != nil {
		zap.S().Errorw("failed to close connection", "error", err)
	}

	// Write close frame
	err := w.c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(int(code), code.String()))
	err = multierror.Append(err, w.c.Close()).ErrorOrNil()
	if err != nil {
		zap.S().Errorw("failed to close connection", "error", err)
	}
}

func (w *WebSocket) write(msg events.Message[json.RawMessage]) error {
	w.writeMtx.Lock()
	defer w.writeMtx.Unlock()

	if err := w.c.WriteJSON(msg); err != nil {
		return err
	}
	return nil
}

func (w *WebSocket) Events() client.EventMap {
	return w.evm
}

func (w *WebSocket) Digest() client.EventDigest {
	return w.dig
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
	msg := events.NewMessage(events.OpcodeError, events.ErrorPayload{
		Message: txt,
		Fields:  fields,
	})
	if err := w.c.WriteJSON(msg); err != nil {
		zap.S().Errorw("failed to write an error message to the socket", "error", err)
	}
}

func (*WebSocket) SetWriter(w *bufio.Writer) {
	zap.S().Fatalw("called SetWriter() on a WebSocket connection")
}
