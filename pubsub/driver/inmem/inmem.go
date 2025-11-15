package inmem

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/infigaming-com/go-common/pubsub"
)

type Transport struct {
	mu   sync.RWMutex
	subs map[string][]*subscription
	seq  int64
}

type subscription struct {
	ctx    context.Context
	cancel context.CancelFunc
	msgs   chan *pubsub.TransportMessage
}

func New() *Transport {
	return &Transport{subs: map[string][]*subscription{}}
}

func (t *Transport) Publish(ctx context.Context, topic string, env *pubsub.Envelope) (string, error) {
	if topic == "" {
		return "", errors.New("inmem: topic required")
	}
	if env == nil {
		env = &pubsub.Envelope{}
	}
	msg := t.prepare(env)
	t.mu.RLock()
	defer t.mu.RUnlock()
	subs := t.subs[topic]
	for _, sub := range subs {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-sub.ctx.Done():
		case sub.msgs <- cloneMessage(msg):
		}
	}
	return msg.ID, nil
}

func (t *Transport) Subscribe(ctx context.Context, topic string, opts pubsub.TransportSubscribeOptions, handler pubsub.TransportHandler) error {
	ctx, cancel := context.WithCancel(ctx)
	sub := &subscription{ctx: ctx, cancel: cancel, msgs: make(chan *pubsub.TransportMessage, opts.Parallelism)}
	t.register(topic, sub)
	defer func() {
		t.unregister(topic, sub)
		close(sub.msgs)
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case m, ok := <-sub.msgs:
			if !ok {
				return nil
			}
			if handler != nil {
				if err := handler(ctx, m); err != nil {
					return err
				}
			}
		}
	}
}

func (t *Transport) Close(ctx context.Context) error {
	t.mu.Lock()
	for topic, subs := range t.subs {
		for _, sub := range subs {
			sub.cancel()
		}
		delete(t.subs, topic)
	}
	t.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond):
		return nil
	}
}

func (t *Transport) register(topic string, sub *subscription) {
	t.mu.Lock()
	t.subs[topic] = append(t.subs[topic], sub)
	t.mu.Unlock()
}

func (t *Transport) unregister(topic string, sub *subscription) {
	t.mu.Lock()
	defer t.mu.Unlock()
	subs := t.subs[topic]
	for i, candidate := range subs {
		if candidate == sub {
			subs = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(subs) == 0 {
		delete(t.subs, topic)
	} else {
		t.subs[topic] = subs
	}
}

type messageState struct {
	done     chan struct{}
	mu       sync.Mutex
	doneOnce bool
}

func newState() *messageState {
	return &messageState{done: make(chan struct{})}
}

func (s *messageState) ack() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doneOnce {
		return nil
	}
	s.doneOnce = true
	close(s.done)
	return nil
}

func (s *messageState) nack() error {
	return s.ack()
}

func (s *messageState) extend(time.Duration) error {
	return nil
}

func (t *Transport) prepare(env *pubsub.Envelope) *pubsub.TransportMessage {
	state := newState()
	return &pubsub.TransportMessage{
		Envelope: pubsub.Envelope{
			ID:          fmt.Sprintf("msg-%d", time.Now().UnixNano()),
			Data:        append([]byte(nil), env.Data...),
			Attributes:  clone(env.Attributes),
			OrderingKey: env.OrderingKey,
		},
		ReceivedAt: time.Now(),
		Ack:        state.ack,
		Nack:       state.nack,
		Extend:     state.extend,
		Done:       state.done,
	}
}

func cloneMessage(src *pubsub.TransportMessage) *pubsub.TransportMessage {
	state := newState()
	return &pubsub.TransportMessage{
		Envelope: pubsub.Envelope{
			ID:          src.ID,
			Data:        append([]byte(nil), src.Data...),
			Attributes:  clone(src.Attributes),
			OrderingKey: src.OrderingKey,
			Attempt:     src.Attempt,
		},
		ReceivedAt: src.ReceivedAt,
		Ack:        state.ack,
		Nack:       state.nack,
		Extend:     state.extend,
		Done:       state.done,
	}
}

func clone(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
