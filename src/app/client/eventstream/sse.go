package eventstream

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/SevenTV/Common/events"
	"github.com/SevenTV/Common/structures/v3"
	"github.com/SevenTV/Common/utils"
	"github.com/SevenTV/EventAPI/src/app/client"
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/hashicorp/go-multierror"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

type EventStream struct {
	ctx               *fasthttp.RequestCtx
	seq               int64
	evm               client.EventMap
	writeMtx          sync.Mutex
	writer            *bufio.Writer
	sessionID         []byte
	heartbeatInterval int64
	heartbeatCount    int64
}

func NewSSE(gctx global.Context, ctx *fasthttp.RequestCtx) (client.Connection, error) {
	hbi := gctx.Config().API.HeartbeatInterval
	if hbi == 0 {
		hbi = 45000
	}

	sessionID, err := client.GenerateSessionID(64)
	if err != nil {
		return nil, err
	}

	es := EventStream{
		ctx,
		0,
		client.NewEventMap(make(chan string, 10)),
		sync.Mutex{},
		nil,
		sessionID,
		hbi,
		0,
	}

	return &es, nil
}

// Context implements Connection
func (es *EventStream) Context() context.Context {
	return es.ctx
}

func (*EventStream) Actor() *structures.User {
	return nil
}

func (es *EventStream) Close(code events.CloseCode) {
	_ = es.ctx.Conn().Close()
}

func (*EventStream) Dispatch(t events.EventType, data []byte) error {
	panic("unimplemented")
}

func (*EventStream) Events() client.EventMap {
	panic("unimplemented")
}

func (*EventStream) Digest() client.EventDigest {
	panic("unimplemented")
}

func (es *EventStream) Greet() error {
	msg, err := events.NewMessage(events.OpcodeHello, events.HelloPayload{
		HeartbeatInterval: int64(es.heartbeatInterval),
		SessionID:         hex.EncodeToString(es.sessionID),
	})
	if err != nil {
		return err
	}

	return es.write(msg.ToRaw())
}

func (es *EventStream) Heartbeat() error {
	es.heartbeatCount++
	msg, err := events.NewMessage(events.OpcodeHeartbeat, events.HeartbeatPayload{
		Count: es.heartbeatCount,
	})
	if err != nil {
		return err
	}

	return es.write(msg.ToRaw())
}

func (*EventStream) SendError(txt string, fields map[string]any) {
	panic("unimplemented")
}

func (es *EventStream) write(msg events.Message[json.RawMessage]) error {
	if es.writer == nil {
		return fmt.Errorf("connection not writable")
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	sb := strings.Builder{}
	_, er1 := sb.WriteString(fmt.Sprintf("event: %s\ndata: ", strings.ToLower(msg.Op.String())))
	_, er2 := sb.Write(b)
	_, er3 := sb.WriteString("\n\n")
	if err = multierror.Append(er1, er2, er3).ErrorOrNil(); err != nil {
		return err
	}

	if _, err = es.writer.Write(utils.S2B(sb.String())); err != nil {
		zap.S().Errorw("failed to write to event stream connection", "error", err)
	}
	if err = es.writer.Flush(); err != nil {
		return err
	}

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
