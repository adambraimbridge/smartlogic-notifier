package kafka

import (
	"strings"

	"github.com/Shopify/sarama"
	log "github.com/Sirupsen/logrus"
)

type Clienter interface {
	SendMessage(message FTMessage) error
}

type Client struct {
	brokers  []string
	topic    string
	producer sarama.SyncProducer
}

func NewKafkaClient(brokers string, topic string) (Clienter, error) {
	brokerSlice := strings.Split(brokers, ",")
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 10
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true

	sp, err := sarama.NewSyncProducer(brokerSlice, config)
	if err != nil {
		log.WithError(err).WithField("method", "NewKafkaClient").Error("Error creating the producer")
		return &Client{}, err
	}

	return &Client{
		brokers:  brokerSlice,
		topic:    topic,
		producer: sp,
	}, nil
}

func (c *Client) SendMessage(message FTMessage) error {
	_, _, err := c.producer.SendMessage(&sarama.ProducerMessage{
		Topic: c.topic,
		Value: sarama.StringEncoder(message.Build()),
	})
	if err != nil {
		log.WithError(err).WithField("method", "SendMessage").Error("Error sending a Kafka message")
	}
	return err
}
