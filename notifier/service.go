package notifier

import (
	"time"

	"github.com/Financial-Times/smartlogic-notifier/kafka"
	"github.com/Financial-Times/smartlogic-notifier/smartlogic"
	log "github.com/Sirupsen/logrus"
)

type Service struct {
	kafka      kafka.Client
	smartlogic smartlogic.Client
}

func NewNotifierService(kafka kafka.Client, smartlogic smartlogic.Client) *Service {
	return &Service{
		kafka:      kafka,
		smartlogic: smartlogic,
	}
}

func (s *Service) GetConcept(uuid string) ([]byte, error) {
	return s.smartlogic.GetConcept(uuid)
}

func (s *Service) Notify(lastChangeDateString string) error {
	lastChange, err := time.Parse("2006-01-02T15:04:05.000Z", lastChangeDateString)
	if err != nil {
		return err
	}
	log.Debugf("lastChange: %v", lastChange)

	changedConcepts, err := s.smartlogic.GetChangedConceptList(lastChange)
	if err != nil {
		return err
	}
	log.Debugf("changedConcepts: %v", changedConcepts)

	return s.ForceNotify(changedConcepts)
}

func (s *Service) ForceNotify(UUIDs []string) error {

	for _, conceptUUID := range UUIDs {
		concept, err := s.smartlogic.GetConcept(conceptUUID)
		if err != nil {
			return err
		}

		message := kafka.NewFTMessage(map[string]string{}, string(concept))
		err = s.kafka.SendMessage(message)
		if err != nil {
			return err
		}
	}
	return nil
}
