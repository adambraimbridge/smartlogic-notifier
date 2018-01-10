package notifier

import (
	"fmt"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
	"time"
)

type HealthService struct {
	config   *config
	notifier Servicer
	Checks   []fthealth.Check
}

type config struct {
	appSystemCode string
	appName       string
	description          string
}

func NewHealthService(notifier Servicer, appSystemCode string, appName string, description string) *HealthService {
	service := &HealthService{
		config: &config{
			appSystemCode: appSystemCode,
			appName:       appName,
			description:          description,
		},
		notifier: notifier,
	}
	service.Checks = []fthealth.Check{
		service.smartlogicHealthCheck(),
	}
	return service
}

func (svc *HealthService) HealthcheckHandler() fthealth.TimedHealthCheck{
	return fthealth.TimedHealthCheck{
		HealthCheck: fthealth.HealthCheck{
			SystemCode: svc.config.appSystemCode,
			Name: svc.config.appName,
			Description: svc.config.description,
			Checks: svc.Checks,
		},
		Timeout: 10 * time.Second,
	}
}

func (svc *HealthService) smartlogicHealthCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   "Editorial updates of concepts will not be written into UPP",
		Name:             "Check connectivity to Smartlogic",
		PanicGuide:       fmt.Sprintf("https://dewey.ft.com/%s.html", svc.config.appSystemCode),
		Severity:         3,
		TechnicalSummary: `Check that Smartlogic is healthy and the API is accessible.  If it is, restart this service.`,
		Checker: svc.smartlogicCheck,
	}
}

func (svc *HealthService) smartlogicCheck()(string, error){
	_, err := svc.notifier.GetConcept("healthcheck-concept")
	if err != nil {
		return "Concept couldn't be retrieved.", err
	}
	return "", nil
}


func (svc *HealthService) GtgCheck() gtg.StatusChecker {
	return gtg.FailFastParallelCheck([]gtg.StatusChecker{
		func()(gtg.Status){
			if _, err := svc.smartlogicCheck(); err != nil {
				return gtg.Status{GoodToGo: false, Message: err.Error()}
			}
			return gtg.Status{GoodToGo: true}
		},
	})
}
