package notifier

import (
	"errors"
	"time"
)

type MockService struct {
	getConcept  func(string) ([]byte, error)
	notify      func(time.Time, string) error
	forceNotify func([]string, string) error
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
