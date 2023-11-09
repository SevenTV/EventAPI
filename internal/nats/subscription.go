package nats

import "sync"

var (
	mx       sync.Mutex
	subjects map[string][]*Subscription // subject as key, list of subscriptions as value
	sessions map[string][]string        // sessionID as key, list of subscribed subjects as value
)

type Subscription struct {
	Ch        chan []byte
	sessionID string
}

func NewSub(sessionID string) *Subscription {
	return &Subscription{
		Ch:        make(chan []byte, 10),
		sessionID: sessionID,
	}
}

func (s *Subscription) Subscribe(subscribeTo ...string) {
	mx.Lock()
	defer mx.Unlock()
	for _, join := range subscribeTo {
		if isSubscribed(s.sessionID, join) {
			continue
		}
		subjects[join] = append(subjects[join], s)
		sessions[s.sessionID] = append(sessions[s.sessionID], join)
	}
}

func (s *Subscription) Unsubscribe(unsubscribeFrom ...string) {
	mx.Lock()
	defer mx.Unlock()
	subs, ok := sessions[s.sessionID]
	if !ok {
		return
	}

	for _, unsub := range unsubscribeFrom {
		for i, subject := range subs {
			if subject != unsub {
				continue
			}

			sessions[s.sessionID] = removeSubject(subs, i)

			subscriptions, ok := subjects[subject]
			if !ok {
				continue
			}

			for o, sub := range subscriptions {
				if sub.sessionID != s.sessionID {
					continue
				}

				subjects[subject] = removeSubscription(subscriptions, o)
				break
			}

			if len(subjects[subject]) == 0 {
				delete(subjects, subject)
			}

			break
		}
	}
}

func (s *Subscription) Close() {
	mx.Lock()
	defer mx.Unlock()
	subs, ok := sessions[s.sessionID]
	if !ok {
		return
	}

	for _, subject := range subs {
		subscriptions, ok := subjects[subject]
		if !ok {
			continue
		}
		for i, sub := range subscriptions {
			if sub.sessionID == s.sessionID {
				subjects[subject] = removeSubscription(subscriptions, i)
				break
			}
		}

		if len(subjects[subject]) == 0 {
			delete(subjects, subject)
		}
	}

	delete(sessions, s.sessionID)
}

func removeSubject(subs []string, i int) []string {
	subs[i] = subs[len(subs)-1]
	return subs[:len(subs)-1]
}

func removeSubscription(subs []*Subscription, i int) []*Subscription {
	subs[i] = subs[len(subs)-1]
	return subs[:len(subs)-1]
}

func isSubscribed(sessionID, subject string) bool {
	for _, session := range subjects[subject] {
		if session.sessionID == sessionID {
			return true
		}
	}
	return false
}
