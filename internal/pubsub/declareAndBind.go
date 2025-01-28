package pubsub

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType int

const (
	Durable SimpleQueueType = iota
	Transient
)

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	mq_chan, _ := conn.Channel()

	dlx_tbl := amqp.Table{
		"x-dead-letter-exchange": "peril_dlx",
	}
	que, err := mq_chan.QueueDeclare(queueName, simpleQueueType == Durable, simpleQueueType == Transient, simpleQueueType == Transient, false, dlx_tbl)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	err = mq_chan.QueueBind(que.Name, key, exchange, false, nil)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	return mq_chan, que, nil
}
