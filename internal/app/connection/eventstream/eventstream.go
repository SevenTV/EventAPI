package eventstream

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/seventv/api/data/events"
	"github.com/seventv/common/structures/v3"
	"github.com/seventv/common/utils"
	"go.uber.org/zap"

	client "github.com/seventv/eventapi/internal/app/connection"
	"github.com/seventv/eventapi/internal/global"
)

type EventStream struct {
	r                 *http.Request
	ctx               context.Context
	gctx              global.Context
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

func NewEventStream(gctx global.Context, r *http.Request) (client.Connection, error) {
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
		r:                 r,
		ctx:               lctx,
		gctx:              gctx,
		cancel:            cancel,
		seq:               0,
		evm:               client.NewEventMap(string(sessionID)),
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

func (es *EventStream) Greet(gctx global.Context) error {
	msg := events.NewMessage(events.OpcodeHello, events.HelloPayload{
		HeartbeatInterval: uint32(es.heartbeatInterval),
		SessionID:         hex.EncodeToString(es.sessionID),
		SubscriptionLimit: es.subscriptionLimit,
		Instance: events.HelloPayloadInstanceInfo{
			Name:       gctx.Config().Pod.Name,
			Population: gctx.Inst().ConcurrencyValue,
		},
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
	es.evm.Destroy(es.gctx)
}

func SetEventStreamHeaders(w http.ResponseWriter) {
	w.Header().Set("Connection", "close")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("X-Accel-Buffering", "no")

	w.WriteHeader(http.StatusOK)
}
