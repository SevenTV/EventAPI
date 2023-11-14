package nats

import (
	"strings"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func handleMessage(msg *nats.Msg) {
	subject := strings.TrimLeft(msg.Subject, baseSubject+".")
	mx.Lock()
	defer mx.Unlock()
	subs := subjects[subject]
	for _, sub := range subs {
		select {
		case sub.Ch <- msg.Data:
		default:
			zap.S().Debug("channel blocked dropping message: ", msg.Subject)
		}
	}
}
