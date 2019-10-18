package notifier

import (
	"errors"
	"sync"
	"time"

	"github.com/Financial-Times/kafka-client-go/kafka"
)

type mockSmartlogicClient struct {
	concepts                  map[string]string
	getChangedConceptListFunc func(changeDate time.Time) ([]string, error)
}

func (sl *mockSmartlogicClient) AccessToken() string {
	return "access-token"
}

func (sl *mockSmartlogicClient) GetConcept(uuid string) ([]byte, error) {
	c, ok := sl.concepts[uuid]
	if !ok {
		return nil, errors.New("can't find concept")
	}
	return []byte(c), nil
}

func (sl *mockSmartlogicClient) GetChangedConceptList(changeDate time.Time) ([]string, error) {
	if sl.getChangedConceptListFunc != nil {
		return sl.getChangedConceptListFunc(changeDate)
	}
	return nil, errors.New("not implemented")
}

type mockKafkaClient struct {
	mu        sync.Mutex
	sentCount int
}

func (kf *mockKafkaClient) ConnectivityCheck() error {
	return nil
}

func (kf *mockKafkaClient) SendMessage(message kafka.FTMessage) error {
	kf.mu.Lock()
	defer kf.mu.Unlock()

	kf.sentCount++
	return nil
}

func (kf *mockKafkaClient) Shutdown() {
}

func (kf *mockKafkaClient) getSentCount() int {
	kf.mu.Lock()
	defer kf.mu.Unlock()
	return kf.sentCount
}

type mockService struct {
	getConcept             func(string) ([]byte, error)
	notify                 func(time.Time, string) error
	forceNotify            func([]string, string) error
	checkKafkaConnectivity func() error
}

func (s *mockService) GetConcept(uuid string) ([]byte, error) {
	if s.getConcept != nil {
		return s.getConcept(uuid)
	}
	return nil, errors.New("not implemented")
}

func (s *mockService) Notify(lastChange time.Time, transactionID string) error {
	if s.notify != nil {
		return s.notify(lastChange, transactionID)
	}
	return errors.New("not implemented")
}

func (s *mockService) ForceNotify(uuids []string, transactionID string) error {
	if s.forceNotify != nil {
		return s.forceNotify(uuids, transactionID)
	}
	return errors.New("not implemented")
}

func (s *mockService) CheckKafkaConnectivity() error {
	if s.checkKafkaConnectivity != nil {
		return s.checkKafkaConnectivity()
	}
	return errors.New("not implemented")
}
