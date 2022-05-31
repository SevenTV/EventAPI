package app

import (
	"encoding/json"

	"github.com/SevenTV/Common/errors"
	"github.com/SevenTV/Common/events"
	"github.com/SevenTV/Common/structures/v3"
	"github.com/SevenTV/Common/utils"
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/google/uuid"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func (s Server) HandleSessionMutation(gctx global.Context) {
	s.router.PUT("/v3/sessions/{sid}/events/{event}", func(ctx *fasthttp.RequestCtx) {
		sid := ctx.UserValue("sid").(string)
		evt := ctx.UserValue("event").(string)

		// Parse request body
		body := SessionMutationEventPut{}
		if err := json.Unmarshal(ctx.Request.Body(), &body); err != nil {
			DoErrorResponse(ctx, errors.ErrInvalidRequest().SetDetail(err.Error()))
			return
		}

		reqID := uuid.New().String()
		b, _ := json.Marshal(&events.SessionMutation{
			RequestID: reqID,
			SessionID: sid,
			Events: []events.SessionMutationEvent{{
				Action:  structures.ListItemActionAdd,
				Type:    events.EventType(evt),
				Targets: body.Targets,
			}},
		})
		gctx.Inst().Redis.RawClient().Publish(ctx, "events:session_mutation", utils.B2S(b))

		j, _ := json.Marshal(SessionMutationResponse{
			RequestID: reqID,
		})
		ctx.SetBody(j)
		ctx.SetContentType("application/json")
		ctx.SetStatusCode(fasthttp.StatusOK)
	})

	ch := make(chan string, 10)
	go gctx.Inst().Redis.Subscribe(gctx, ch, "events:session_mutation")
	go func() {
		defer close(ch)
		for {
			select {
			case <-gctx.Done():
				return
			case d := <-ch:
				m := events.SessionMutation{}
				if err := json.Unmarshal(utils.S2B(d), &m); err != nil {
					zap.S().Errorw("couldn't decode session mutation message",
						"error", err,
					)
					continue
				}

				conn, ok := s.conns.Load(m.SessionID)
				if !ok {
					continue
				}

				// Handle event changes
				for _, ev := range m.Events {
					_, _ = conn.Events().Subscribe(gctx, ev.Type, ev.Targets)

					// Publish update
					ackMsg := events.NewMessage(events.OpcodeAck, events.AckPayload{
						RequestID: m.RequestID,
						Data: map[string]any{
							"action":     ev.Action,
							"event_type": ev.Type,
						},
					})
					conn.Digest().Ack.Publish(gctx, ackMsg.ToRaw())
				}

			}
		}
	}()
}

type SessionMutationEventPut struct {
	Targets []string `json:"targets"`
}

type SessionMutationResponse struct {
	RequestID string `json:"request_id"`
}
