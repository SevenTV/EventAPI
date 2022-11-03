package eventstream

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/fasthttp/router"
	"github.com/hashicorp/go-multierror"
	"github.com/seventv/api/data/events"
	"github.com/seventv/common/structures/v3"
	"github.com/seventv/common/utils"
	client "github.com/seventv/eventapi/internal/app/connection"
	"github.com/seventv/eventapi/internal/global"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

type EventStream struct {
	c                 *fasthttp.RequestCtx
	ctx               context.Context
	cancel            context.CancelFunc
	seq               int64
	evm               client.EventMap
	dig               client.EventDigest
	writeMtx          sync.Mutex
	writer            *bufio.Writer
	sessionID         []byte
	heartbeatInterval int64
	heartbeatCount    int64
}

func NewEventStream(gctx global.Context, c *fasthttp.RequestCtx, dig client.EventDigest, r *router.Router) (client.Connection, error) {
	hbi := gctx.Config().API.HeartbeatInterval
	if hbi == 0 {
		hbi = 45000
	}

	sessionID, err := client.GenerateSessionID(32)
	if err != nil {
		return nil, err
	}

	lctx, cancel := context.WithCancel(gctx)
	es := EventStream{
		c:                 c,
		ctx:               lctx,
		cancel:            cancel,
		seq:               0,
		evm:               client.NewEventMap(make(chan string, 10)),
		dig:               dig,
		writeMtx:          sync.Mutex{},
		writer:            nil,
		sessionID:         sessionID,
		heartbeatInterval: hbi,
		heartbeatCount:    0,
	}

	return &es, nil
}

// Context implements Connection
func (es *EventStream) Context() context.Context {
	return es.ctx
}

func (es *EventStream) SessionID() string {
	return hex.EncodeToString(es.sessionID)
}

func (*EventStream) Actor() *structures.User {
	return nil
}

func (es *EventStream) Close(code events.CloseCode, write bool) {
	msg := events.NewMessage(events.OpcodeEndOfStream, events.EndOfStreamPayload{
		Code:    code,
		Message: code.String(),
	})

	if err := es.write(msg.ToRaw()); err != nil {
		zap.S().Errorw("failed to write end of stream event to closing connection", "error", err)
	}
	es.cancel()
}

func (es *EventStream) Events() client.EventMap {
	return es.evm
}

func (es *EventStream) Digest() client.EventDigest {
	return es.dig
}

func (es *EventStream) Greet() error {
	msg := events.NewMessage(events.OpcodeHello, events.HelloPayload{
		HeartbeatInterval: int64(es.heartbeatInterval),
		SessionID:         hex.EncodeToString(es.sessionID),
	})

	return es.write(msg.ToRaw())
}

func (es *EventStream) Heartbeat() error {
	es.heartbeatCount++
	msg := events.NewMessage(events.OpcodeHeartbeat, events.HeartbeatPayload{
		Count: es.heartbeatCount,
	})

	return es.write(msg.ToRaw())
}

func (es *EventStream) SendError(txt string, fields map[string]any) {
	if fields == nil {
		fields = make(map[string]any)
	}
	msg := events.NewMessage(events.OpcodeError, events.ErrorPayload{
		Message: txt,
		Fields:  fields,
	})

	if err := es.write(msg.ToRaw()); err != nil {
		zap.S().Errorw("failed to write an error message to the socket", "error", err)
	}
}

func (es *EventStream) write(msg events.Message[json.RawMessage]) error {
	es.writeMtx.Lock()
	defer es.writeMtx.Unlock()

	if es.writer == nil {
		return fmt.Errorf("connection not writable")
	}
	b, err := json.Marshal(msg.Data)
	if err != nil {
		return err
	}

	sb := strings.Builder{}
	_, er1 := sb.WriteString(fmt.Sprintf("type: %s\ndata: ", strings.ToLower(msg.Op.String())))
	_, er2 := sb.Write(b)
	_, er3 := sb.WriteString(fmt.Sprintf("\nid: %d", es.seq))
	_, er4 := sb.WriteString("\n\n")
	if err = multierror.Append(er1, er2, er3, er4).ErrorOrNil(); err != nil {
		return err
	}

	if _, err = es.writer.Write(utils.S2B(sb.String())); err != nil {
		zap.S().Errorw("failed to write to event stream connection", "error", err)
	}
	if err = es.writer.Flush(); err != nil {
		return err
	}

	es.seq++
	return nil
}

// SetWriter implements Connection
func (es *EventStream) SetWriter(w *bufio.Writer) {
	es.writer = w
}

func SetupEventStream(ctx *fasthttp.RequestCtx, writer fasthttp.StreamWriter) {
	ctx.SetStatusCode(200)

	ctx.Response.ImmediateHeaderFlush = true
	ctx.Response.SetConnectionClose()

	ctx.Response.Header.Set("Content-Type", "text/event-stream")
	ctx.Response.Header.Set("Cache-Control", "no-cache")
	ctx.Response.Header.Set("Transfer-Encoding", "chunked")
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	ctx.Response.Header.Set("Access-Control-Allow-Headers", "Cache-Control")
	ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
	ctx.Response.Header.Set("X-Accel-Buffering", "no")

	ctx.SetBodyStreamWriter(writer)
}
