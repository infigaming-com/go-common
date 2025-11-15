package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

type Encoder interface {
	Encode(ctx context.Context, v any) (*Envelope, error)
}

type Decoder interface {
	Decode(ctx context.Context, data []byte, into any) error
}

type jsonCodec struct{}

func (jsonCodec) Encode(_ context.Context, v any) (*Envelope, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &Envelope{Data: bytes}, nil
}

func (jsonCodec) Decode(_ context.Context, data []byte, into any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, into)
}

type Handler interface {
	Handle(context.Context, *Message) error
}

type HandlerFunc func(context.Context, *Message) error

func (f HandlerFunc) Handle(ctx context.Context, m *Message) error {
	return f(ctx, m)
}

var ErrDropped = errors.New("pubsub: dropped message")

type permanentError struct{ Err error }

func (p permanentError) Error() string { return p.Err.Error() }

func (p permanentError) Unwrap() error { return p.Err }

func ErrPermanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{Err: err}
}

type Message struct {
	id         string
	data       []byte
	attributes map[string]string
	attempt    int
	receivedAt time.Time
	ackFn      func() error
	nackFn     func() error
	extendFn   func(time.Duration) error
	done       <-chan struct{}
	acksync    sync.Once
	nacksync   sync.Once
	decoder    Decoder
}

func newMessage(src *TransportMessage, decoder Decoder) *Message {
	return &Message{
		id:         src.ID,
		data:       src.Data,
		attributes: cloneMap(src.Attributes),
		attempt:    src.Attempt,
		receivedAt: src.ReceivedAt,
		ackFn:      src.Ack,
		nackFn:     src.Nack,
		extendFn:   src.Extend,
		done:       src.Done,
		decoder:    decoder,
	}
}

func (m *Message) ID() string { return m.id }

func (m *Message) Attempt() int { return m.attempt }

func (m *Message) Attributes() map[string]string { return cloneMap(m.attributes) }

func (m *Message) ReceivedAt() time.Time { return m.receivedAt }

func (m *Message) Data() []byte { return append([]byte(nil), m.data...) }

func (m *Message) Ack() error {
	var err error
	m.acksync.Do(func() {
		if m.ackFn != nil {
			err = m.ackFn()
		}
	})
	return err
}

func (m *Message) Nack() error {
	var err error
	m.nacksync.Do(func() {
		if m.nackFn != nil {
			err = m.nackFn()
		}
	})
	return err
}

func (m *Message) Extend(deadline time.Duration) error {
	if m.extendFn == nil || deadline <= 0 {
		return nil
	}
	return m.extendFn(deadline)
}

func (m *Message) Done() <-chan struct{} { return m.done }

func (m *Message) Decode(ctx context.Context, into any) error {
	if m.decoder == nil {
		return jsonCodec{}.Decode(ctx, m.data, into)
	}
	return m.decoder.Decode(ctx, m.data, into)
}

func cloneMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(src))
	for k, v := range src {
		cloned[k] = v
	}
	return cloned
}
