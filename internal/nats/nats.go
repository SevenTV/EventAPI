package nats

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func Init(url string, subject string) error {
	var err error
	conn, err = nats.Connect(url)
	if err != nil {
		return err
	}
	// wait for connection to be clear
	conn.Flush()

	subscription, err = conn.Subscribe(fmt.Sprintf("%v.>", subject), handleMessage)

	baseSubject = subject
	subjects = make(map[string][]*Subscription)
	sessions = make(map[string][]string)
	return err
}

func Close() {
	err := subscription.Unsubscribe()
	if err != nil {
		zap.S().Errorw("closing NATS", "error", err)
	}
	err = conn.Flush()
	if err != nil {
		zap.S().Errorw("closing NATS", "error", err)
	}
	conn.Close()
}

var (
	subscription *nats.Subscription
	conn         *nats.Conn
	baseSubject  string
)
