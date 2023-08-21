package nats

import (
	"fmt"

	"github.com/nats-io/nats.go"
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
	subscription.Unsubscribe()
	conn.Flush()
	conn.Close()
}

var (
	subscription *nats.Subscription
	conn         *nats.Conn
	baseSubject  string
)
