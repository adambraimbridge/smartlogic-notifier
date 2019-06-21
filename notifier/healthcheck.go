package notifier

import (
	"errors"
	"fmt"
	"sync"
	"time"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/service-status-go/gtg"
	log "github.com/sirupsen/logrus"
)

// FTUuid is the uuid of the organisation Financial Times in Smartlogic
const FTUuid = "b1a492d9-dcfe-43f8-8072-17b4618a78fd"

// HealthService is responsible for gtg and health checks
type HealthService struct {
	sync.RWMutex
	config            *config
	notifier          Servicer
	Checks            []fthealth.Check
	checkSuccessCache bool
}

type config struct {
	appSystemCode               string
	appName                     string
	description                 string
	smartlogicModel             string
	healthcheckSuccessCacheTime time.Duration
}

// HealthService initialises the HealthCheck service but doesn't start the updating of the health check result
func NewHealthService(notifier Servicer, appSystemCode string, appName string, description string, smartlogicModel string, healthcheckSuccessCacheTime time.Duration) *HealthService {
	service := &HealthService{
		config: &config{
			appSystemCode:               appSystemCode,
			appName:                     appName,
			description:                 description,
			smartlogicModel:             smartlogicModel,
			healthcheckSuccessCacheTime: healthcheckSuccessCacheTime,
		},
		notifier: notifier,
	}
	service.Checks = []fthealth.Check{
		service.smartlogicHealthCheck(),
	}
	return service
}

// Start starts separate go routine responsible for updating the cached result of the gtg/health check
func (svc *HealthService) Start() {
	go func() {
		// perform connectivity check and cache the result
		err := svc.updateSmartlogicSuccessCache()
		if err != nil {
			log.WithError(err).Error("could not perform Smartlogic connectivity check")
		}

		c := time.Tick(svc.config.healthcheckSuccessCacheTime)
		for range c {
			err := svc.updateSmartlogicSuccessCache()
			if err != nil {
				log.WithError(err).Error("could not perform latest Smartlogic connectivity check")
			}
		}
	}()
}

func (svc *HealthService) updateSmartlogicSuccessCache() error {
	success, err := svc.smartlogicCheck()
	if err != nil {
		svc.setCheckSuccessCache(false)
		return err
	}
	svc.setCheckSuccessCache(success)
	return nil
}

// HealthcheckHandler is resposible for __health endpoint
func (svc *HealthService) HealthcheckHandler() fthealth.TimedHealthCheck {
	return fthealth.TimedHealthCheck{
		HealthCheck: fthealth.HealthCheck{
			SystemCode:  svc.config.appSystemCode,
			Name:        svc.config.appName,
			Description: svc.config.description,
			Checks:      svc.Checks,
		},
		Timeout: 10 * time.Second,
	}
}

func (svc *HealthService) smartlogicHealthCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   "Editorial updates of concepts will not be written into UPP",
		Name:             fmt.Sprintf("Check connectivity to Smartlogic model %s", svc.config.smartlogicModel),
		PanicGuide:       fmt.Sprintf("https://dewey.ft.com/%s.html", svc.config.appSystemCode),
		Severity:         3,
		TechnicalSummary: `Check that Smartlogic is healthy and the API is accessible.  If it is, restart this service.`,
		Checker:          svc.smartlogicConnectivityCheck,
	}
}

// smartlogicConnectivityCheck always returns the cached result for the Smartlogic connectivity check
func (svc *HealthService) smartlogicConnectivityCheck() (string, error) {
	if !svc.getCheckSuccessCache() {
		msg := "latest Smartlogic connectivity check is unsuccessful"
		log.Error(msg)
		return msg, errors.New(msg)
	}
	return "", nil
}

// smartlogicCheck checks the UUID for Financial Times organisation as it should always exist in the Smartlogic ontology
func (svc *HealthService) smartlogicCheck() (bool, error) {
	_, err := svc.notifier.GetConcept(FTUuid)
	if err != nil {
		log.WithError(err).Error("FT organisation concept couldn't be retrieved")
		return false, err
	}
	return true, nil
}

// HealthcheckHandler is responsible for __gtg endpoint
func (svc *HealthService) GtgCheck() gtg.StatusChecker {
	return gtg.FailFastParallelCheck([]gtg.StatusChecker{
		func() gtg.Status {
			if _, err := svc.smartlogicConnectivityCheck(); err != nil {
				return gtg.Status{GoodToGo: false, Message: err.Error()}
			}
			return gtg.Status{GoodToGo: true}
		},
	})
}

func (svc *HealthService) getCheckSuccessCache() bool {
	svc.RLock()
	defer svc.RUnlock()
	return svc.checkSuccessCache
}

func (svc *HealthService) setCheckSuccessCache(val bool) {
	svc.Lock()
	defer svc.Unlock()
	svc.checkSuccessCache = val
}
