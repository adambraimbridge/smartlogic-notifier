package notifier

import (
	"errors"
	"time"

	"github.com/Financial-Times/kafka-client-go/kafka"
	"github.com/Financial-Times/smartlogic-notifier/smartlogic"
)

type mockSmartlogicClient struct {
	concepts        map[string]string
	changedConcepts []string
}

func NewMockSmartlogicClient(concepts map[string]string, changedConcepts []string) smartlogic.Clienter {
	return &mockSmartlogicClient{
		concepts:        concepts,
		changedConcepts: changedConcepts,
	}
}

func (sl *mockSmartlogicClient) AccessToken() string {
	return "access-token"
}

func (sl *mockSmartlogicClient) GetConcept(uuid string) ([]byte, error) {
	c, ok := sl.concepts[uuid]
	if !ok {
		return nil, errors.New("Can't find concept")
	}
	return []byte(c), nil
}

func (sl *mockSmartlogicClient) GetChangedConceptList(changeDate time.Time) ([]string, error) {
	return sl.changedConcepts, nil
}

type mockKafkaClient struct {
	sentCount int
}

func (kf *mockKafkaClient) ConnectivityCheck() error {
	return nil
}

func (kf *mockKafkaClient) SendMessage(message kafka.FTMessage) error {
	kf.sentCount++
	return nil
}

func (kf *mockKafkaClient) Shutdown() {
}
