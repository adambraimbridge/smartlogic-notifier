package notifier

import (
	"errors"
	"testing"
	"time"

	"github.com/Financial-Times/kafka-client-go/kafka"
	"github.com/Financial-Times/smartlogic-notifier/smartlogic"
	"github.com/stretchr/testify/assert"
)

func TestNewNotifierService(t *testing.T) {
	kc := &mockKafkaClient{}
	sl := &mockSmartlogicClient{}

	service := NewNotifierService(kc, sl)
	assert.IsType(t, &Service{}, service)
}

func TestService_GetConcept(t *testing.T) {
	kc := &mockKafkaClient{}
	sl := NewMockSmartlogicClient(map[string]string{
		"uuid1": "concept1",
		"uuid2": "concept2",
	}, []string{})
	service := NewNotifierService(kc, sl)

	concept, err := service.GetConcept("uuid2")
	assert.NoError(t, err)
	assert.EqualValues(t, "concept2", string(concept))
}

func TestService_Notify(t *testing.T) {
	kc := &mockKafkaClient{}
	sl := NewMockSmartlogicClient(map[string]string{
		"uuid1": "concept1",
		"uuid2": "concept2",
	}, []string{"uuid2"})
	service := NewNotifierService(kc, sl)

	err := service.Notify(time.Now(), "transactionID")

	assert.NoError(t, err)
	assert.Equal(t, 1, kc.sentCount)
}

func TestService_ForceNotify(t *testing.T) {
	kc := &mockKafkaClient{}
	sl := NewMockSmartlogicClient(map[string]string{
		"uuid1": "concept1",
		"uuid2": "concept2",
	}, []string{})
	service := NewNotifierService(kc, sl)

	err := service.ForceNotify([]string{"uuid1"}, "transactionID")

	assert.NoError(t, err)
	assert.Equal(t, 1, kc.sentCount)
}

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

func (kf *mockKafkaClient) SendMessage(message kafka.FTMessage) error {
	kf.sentCount++
	return nil
}
