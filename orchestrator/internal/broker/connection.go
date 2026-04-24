package broker

import (
	"log"

	"github.com/streadway/amqp"
)

type Broker struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewBroker(url string) (*Broker, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}
	return &Broker{conn: conn, channel: ch}, nil
}

func (b *Broker) DeclareQueue(name string) (amqp.Queue, error) {
	return b.channel.QueueDeclare(
		name,  // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
}

func (b *Broker) Publish(queueName string, body []byte) error {
	return b.channel.Publish(
		"",        // exchange
		queueName, // routing key
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)
}

func (b *Broker) Consume(queueName string, handler func([]byte) error) error {
	msgs, err := b.channel.Consume(
		queueName,
		"",    // consumer tag
		false, // auto-ack (мы будем ack вручную после успешной обработки)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return err
	}
	go func() {
		for msg := range msgs {
			if err := handler(msg.Body); err != nil {
				log.Printf("处理失败: %v", err)
				// Отправляем в dead-letter или просто nack с requeue
				msg.Nack(false, true) // requeue
			} else {
				msg.Ack(false)
			}
		}
	}()
	return nil
}

func (b *Broker) Close() error {
	b.channel.Close()
	return b.conn.Close()
}
