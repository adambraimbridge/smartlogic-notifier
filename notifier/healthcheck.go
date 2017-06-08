package notifier

import (
	"fmt"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
)

type HealthService struct {
	config   *config
	notifier Servicer
	Checks   []fthealth.Check
}

type config struct {
	appSystemCode string
	appName       string
	port          string
}

func NewHealthService(notifier Servicer, appSystemCode string, appName string, port string) *HealthService {
	service := &HealthService{
		config: &config{
			appSystemCode: appSystemCode,
			appName:       appName,
			port:          port,
		},
		notifier: notifier,
	}
	service.Checks = []fthealth.Check{
		service.smartlogicHealthCheck(),
	}
	return service
}

func (svc *HealthService) smartlogicHealthCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   "Editorial updates of concepts will not be written into UPP",
		Name:             "Check connectivity to Smartlogic",
		PanicGuide:       fmt.Sprintf("https://dewey.ft.com/%s.html", svc.config.appSystemCode),
		Severity:         3,
		TechnicalSummary: `Check that Smartlogic is healthy and the API is accessible.  If it is, restart this service.`,
		Checker: func() (string, error) {
			_, err := svc.notifier.GetConcept("healthcheck-concept")
			if err != nil {
				return "Concept couldn't be retrieved.", err
			}
			return "", nil
		},
	}
}

func (svc *HealthService) GtgCheck() gtg.Status {
	for _, check := range svc.Checks {
		if _, err := check.Checker(); err != nil {
			return gtg.Status{GoodToGo: false, Message: err.Error()}
		}
	}
	return gtg.Status{GoodToGo: true}
}
