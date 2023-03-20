package eventstream

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

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
	handler           client.Handler
	evm               *client.EventMap
	cache             client.Cache
	evbuf             client.EventBuffer
	writeMtx          *sync.Mutex
	writer            *bufio.Writer
	ready             chan struct{}
	readyOnce         sync.Once
	sessionID         []byte
	heartbeatInterval uint32
	heartbeatCount    uint64
	subscriptionLimit int32
}

func NewEventStream(gctx global.Context, c *fasthttp.RequestCtx, r *router.Router) (client.Connection, error) {
	hbi := gctx.Config().API.HeartbeatInterval
	if hbi == 0 {
		hbi = 45000
	}

	sessionID, err := client.GenerateSessionID(32)
	if err != nil {
		return nil, err
	}

	lctx, cancel := context.WithCancel(context.Background())
	es := &EventStream{
		c:                 c,
		ctx:               lctx,
		cancel:            cancel,
		seq:               0,
		evm:               client.NewEventMap(make(chan string, 16)),
		cache:             client.NewCache(),
		writeMtx:          &sync.Mutex{},
		writer:            nil,
		ready:             make(chan struct{}),
		sessionID:         sessionID,
		heartbeatInterval: hbi,
		heartbeatCount:    0,
		subscriptionLimit: gctx.Config().API.SubscriptionLimit,
	}

	es.handler = client.NewHandler(es)

	return es, nil
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

// Handler implements client.Connection
func (es *EventStream) Handler() client.Handler {
	return es.handler
}

func (es *EventStream) SendClose(code events.CloseCode, after time.Duration) {
	if es.ctx.Err() != nil {
		return
	}

	defer es.Destroy()

	msg := events.NewMessage(events.OpcodeEndOfStream, events.EndOfStreamPayload{
		Code:    code,
		Message: code.String(),
	})

	if err := es.Write(msg.ToRaw()); err != nil {
		zap.S().Errorw("failed to write end of stream event to closing connection", "error", err)
	}

	select {
	case <-es.ctx.Done():
	case <-time.After(after):
	}

}

func (es *EventStream) Events() *client.EventMap {
	return es.evm
}

func (es *EventStream) Cache() client.Cache {
	return es.cache
}

// Buffer implements client.Connection
func (es *EventStream) Buffer() client.EventBuffer {
	return es.evbuf
}

func (es *EventStream) Greet() error {
	msg := events.NewMessage(events.OpcodeHello, events.HelloPayload{
		HeartbeatInterval: uint32(es.heartbeatInterval),
		SessionID:         hex.EncodeToString(es.sessionID),
		SubscriptionLimit: es.subscriptionLimit,
	})

	return es.Write(msg.ToRaw())
}

func (es *EventStream) SendHeartbeat() error {
	es.heartbeatCount++
	msg := events.NewMessage(events.OpcodeHeartbeat, events.HeartbeatPayload{
		Count: es.heartbeatCount,
	})

	return es.Write(msg.ToRaw())
}

// SendAck implements client.Connection
func (es *EventStream) SendAck(cmd events.Opcode, data json.RawMessage) error {
	msg := events.NewMessage(events.OpcodeAck, events.AckPayload{
		Command: cmd.String(),
		Data:    data,
	})

	return es.Write(msg.ToRaw())
}

func (es *EventStream) SendError(txt string, fields map[string]any) {
	if fields == nil {
		fields = make(map[string]any)
	}
	msg := events.NewMessage(events.OpcodeError, events.ErrorPayload{
		Message: txt,
		Fields:  fields,
	})

	if err := es.Write(msg.ToRaw()); err != nil {
		zap.S().Errorw("failed to write an error message to the socket", "error", err)
	}
}

func (es *EventStream) Write(msg events.Message[json.RawMessage]) error {
	if es.writer == nil {
		return fmt.Errorf("connection not writable")
	}
	b, err := json.Marshal(msg.Data)
	if err != nil {
		return err
	}

	sb := strings.Builder{}
	_, er1 := sb.WriteString(fmt.Sprintf("event: %s\ndata: ", strings.ToLower(msg.Op.String())))
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

// Ready implements client.Connection
func (es *EventStream) OnReady() <-chan struct{} {
	return es.ready
}

// Ready implements client.Connection
func (es *EventStream) OnClose() <-chan struct{} {
	return es.ctx.Done()
}

func (es *EventStream) SetReady() {
	es.readyOnce.Do(func() {
		close(es.ready)
	})
}

func (es *EventStream) Destroy() {
	es.cancel()
	es.SetReady()
	es.evm.Destroy()
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
