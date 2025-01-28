package pubsub

import (
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
)

func UnmarshalJSON[T any](data []byte) (*T, error) {
	var result T
	err := json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

type AckType int

const (
	Ack AckType = iota
	NackRequeue
	NackDiscard
)

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType,
	handler func(T) AckType,
) error {
	ch, que, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}
	deli_chan, err := ch.Consume(que.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	// Start a goroutine that ranges over the channel of deliveries, and for each message:
	// Unmarshal the body (raw bytes) of each message delivery into the (generic) T type.
	// Call the given handler function with the unmarshaled message
	// Acknowledge the message with delivery.Ack(false) to remove it from the queue
	go func() {
		for delivery := range deli_chan {
			var result T
			deli_err := json.Unmarshal(delivery.Body, &result)
			if deli_err != nil {
				err = deli_err
				delivery.Nack(false, true)
				break
			}
			ack_type := handler(result)
			switch ack_type {
			case Ack:
				delivery.Ack(false)
			case NackRequeue:
				delivery.Nack(false, true)
			case NackDiscard:
				delivery.Nack(false, false)
			}
		}
	}()
	return nil
}
