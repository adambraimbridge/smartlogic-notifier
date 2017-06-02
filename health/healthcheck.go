package health

import (
	health "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
)

type HealthService struct {
	config *healthConfig
	Checks []health.Check
}

type healthConfig struct {
	appSystemCode string
	appName       string
	port          string
}

func NewHealthService(appSystemCode string, appName string, port string) *HealthService {
	service := &HealthService{config: &healthConfig{
		appSystemCode: appSystemCode,
		appName:       appName,
		port:          port,
	}}
	service.Checks = []health.Check{
		service.sampleCheck(),
	}
	return service
}

func (service *HealthService) sampleCheck() health.Check {
	return health.Check{
		BusinessImpact:   "Sample healthcheck has no impact",
		Name:             "Sample healthcheck",
		PanicGuide:       "https://dewey.ft.com/" + service.config.appSystemCode + ".html",
		Severity:         1,
		TechnicalSummary: "Sample healthcheck has no technical details",
		Checker:          service.sampleChecker,
	}
}

func (service *HealthService) sampleChecker() (string, error) {
	return "Sample is healthy", nil

}

func (service *HealthService) GtgCheck() gtg.Status {
	for _, check := range service.Checks {
		if _, err := check.Checker(); err != nil {
			return gtg.Status{GoodToGo: false, Message: err.Error()}
		}
	}
	return gtg.Status{GoodToGo: true}
}
