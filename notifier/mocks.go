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

type MockService struct {
	getConcept             func(string) ([]byte, error)
	notify                 func(time.Time, string) error
	forceNotify            func([]string, string) error
	checkKafkaConnectivity func() error
}

func (s *MockService) GetConcept(uuid string) ([]byte, error) {
	if s.getConcept != nil {
		return s.getConcept(uuid)
	}
	return nil, errors.New("not implemented")
}

func (s *MockService) Notify(lastChange time.Time, transactionID string) error {
	if s.notify != nil {
		return s.notify(lastChange, transactionID)
	}
	return errors.New("not implemented")
}

func (s *MockService) ForceNotify(uuids []string, transactionID string) error {
	if s.forceNotify != nil {
		return s.forceNotify(uuids, transactionID)
	}
	return errors.New("not implemented")
}

func (s *MockService) CheckKafkaConnectivity() error {
	if s.checkKafkaConnectivity != nil {
		return s.checkKafkaConnectivity()
	}
	return errors.New("not implemented")
}
