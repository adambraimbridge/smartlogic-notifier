package kafka

import (
	"strings"
	"testing"

	"github.com/Shopify/sarama/mocks"
	"github.com/stretchr/testify/assert"
)

const testBrokers = "test1:1,test2:2"
const testTopic = "testTopic"

func NewTestKafkaClient(t *testing.T, brokers string, topic string) (Client, error) {
	msp := mocks.NewSyncProducer(t, nil)
	brokerSlice := strings.Split(brokers, ",")

	msp.ExpectSendMessageAndSucceed()

	return Client{
		brokers:  brokerSlice,
		topic:    topic,
		producer: msp,
	}, nil
}

func Test_NewKafkaClient_BrokerError(t *testing.T) {

	_, err := NewKafkaClient(testBrokers, testTopic)

	assert.Error(t, err)
	//assert.EqualValues(t, []string{"test1:1", "test2:2"}, k.brokers)
	//assert.EqualValues(t, testTopicName, k.topic)
}

func TestClient_SendMessage(t *testing.T) {
	kc, _ := NewTestKafkaClient(t, testBrokers, testTopic)
	kc.SendMessage(NewFTMessage(nil, "Body"))
}
