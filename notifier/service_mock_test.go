package notifier

import (
	"errors"
	"time"
)

type mockService struct {
	concepts  map[string]string
	changes   []string
	err       error
	sentCount int
}

func NewMockService(concepts map[string]string, changes []string, err error) Servicer {
	return &mockService{
		concepts: concepts,
		changes:  changes,
		err:      err,
	}
}

func (s *mockService) GetConcept(uuid string) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}

	c, ok := s.concepts[uuid]
	if !ok {
		return nil, errors.New("Can't find concept")
	}

	return []byte(c), nil
}

func (s *mockService) Notify(lastChange time.Time, transactionID string) error {
	return s.ForceNotify(s.changes, transactionID)
}

func (s *mockService) ForceNotify(UUIDs []string, transactionID string) error {
	for _, v := range UUIDs {
		if _, ok := s.concepts[v]; ok {
			s.sentCount++
		}
	}
	return s.err
}
