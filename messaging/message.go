// messaging/messaging.go
package messaging

type Message struct {
	Topic    string                 // topic name
	Payload  interface{}            // message payload
	Metadata map[string]interface{} // message metadata
}

type KeyGenerator func(msg Message) string
