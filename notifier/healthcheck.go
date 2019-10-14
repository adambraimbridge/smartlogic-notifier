package notifier

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/http-handlers-go/httphandlers"
	"github.com/Financial-Times/service-status-go/gtg"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/gorilla/mux"
	metrics "github.com/rcrowley/go-metrics"
	log "github.com/sirupsen/logrus"
)

// FTUuid is the uuid of the organisation Financial Times in Smartlogic.
const FTUuid = "b1a492d9-dcfe-43f8-8072-17b4618a78fd"

const (
	businessImpact = "Editorial updates of concepts in Smartlogic will not be ingested into UPP"
	panicGuideURL  = "https://runbooks.in.ft.com/smartlogic-notifier"
)

// HealthService is responsible for gtg and health checks.
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

// NewHealthService initialises the HealthCheck service but doesn't start the updating of the health check result.
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
		service.kafkaHealthCheck(),
		service.smartlogicHealthCheck(),
	}
	return service
}

// Start starts separate go routine responsible for updating the cached result of the gtg/health check.
func (hs *HealthService) Start() {
	go func() {
		// perform connectivity check and cache the result
		err := hs.updateSmartlogicSuccessCache()
		if err != nil {
			log.WithError(err).Error("could not perform Smartlogic connectivity check")
		}

		c := time.Tick(hs.config.healthcheckSuccessCacheTime)
		for range c {
			err := hs.updateSmartlogicSuccessCache()
			if err != nil {
				log.WithError(err).Error("could not perform latest Smartlogic connectivity check")
			}
		}
	}()
}

// updateSmartlogicSuccessCache checks the UUID for Financial Times organisation as it should always exist in the Smartlogic ontology
// and based on the success of the check updates the HealthService cache.
func (hs *HealthService) updateSmartlogicSuccessCache() error {
	_, err := hs.notifier.GetConcept(FTUuid)
	if err != nil {
		log.WithError(err).Error("FT organisation concept couldn't be retrieved")
		hs.setCheckSuccessCache(false)
		return err
	}
	hs.setCheckSuccessCache(true)
	return nil
}

// RegisterAdminEndpoints adds the admin endpoints to the given router
func (hs *HealthService) RegisterAdminEndpoints(router *mux.Router) http.Handler {
	router.HandleFunc("/__health", fthealth.Handler(hs.HealthcheckHandler()))
	router.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(hs.GtgCheck()))
	router.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)

	var monitoringRouter http.Handler = router
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(log.StandardLogger(), monitoringRouter)
	monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)

	return monitoringRouter
}

// HealthcheckHandler is resposible for __health endpoint.
func (hs *HealthService) HealthcheckHandler() fthealth.TimedHealthCheck {
	return fthealth.TimedHealthCheck{
		HealthCheck: fthealth.HealthCheck{
			SystemCode:  hs.config.appSystemCode,
			Name:        hs.config.appName,
			Description: hs.config.description,
			Checks:      hs.Checks,
		},
		Timeout: 10 * time.Second,
	}
}

func (hs *HealthService) smartlogicHealthCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   businessImpact,
		Name:             fmt.Sprintf("Check connectivity to Smartlogic model %s", hs.config.smartlogicModel),
		PanicGuide:       panicGuideURL,
		Severity:         3,
		TechnicalSummary: `Check that Smartlogic is healthy and the API is accessible.  If it is, restart this service.`,
		Checker:          hs.smartlogicConnectivityCheck,
	}
}

func (hs *HealthService) kafkaHealthCheck() fthealth.Check {
	return fthealth.Check{
		BusinessImpact:   businessImpact,
		Name:             "Check connectivity to Kafka",
		PanicGuide:       panicGuideURL,
		Severity:         3,
		TechnicalSummary: `Cannot connect to Kafka. Verify that Kafka is healthy in this cluster.`,
		Checker:          hs.checkKafkaConnectivity,
	}
}

// smartlogicConnectivityCheck always returns the cached result for the Smartlogic connectivity check.
func (hs *HealthService) smartlogicConnectivityCheck() (string, error) {
	if !hs.getCheckSuccessCache() {
		msg := "latest Smartlogic connectivity check is unsuccessful"
		log.Error(msg)
		return msg, errors.New(msg)
	}
	return "", nil
}

func (hs *HealthService) checkKafkaConnectivity() (string, error) {
	err := hs.notifier.CheckKafkaConnectivity()
	if err != nil {
		clientError := fmt.Sprint("Error verifying open connection to Kafka")
		log.WithError(err).Error(clientError)
		return "Error connecting with Kafka", errors.New(clientError)
	} else {
		return "Successfully connected to Kafka", nil
	}
}

// GtgCheck is responsible for __gtg endpoint.
func (hs *HealthService) GtgCheck() gtg.StatusChecker {
	var sc []gtg.StatusChecker
	for _, c := range hs.Checks {
		sc = append(sc, gtgCheck(c.Checker))
	}

	return gtg.FailFastParallelCheck(sc)
}

func (hs *HealthService) getCheckSuccessCache() bool {
	hs.RLock()
	defer hs.RUnlock()
	return hs.checkSuccessCache
}

func (hs *HealthService) setCheckSuccessCache(val bool) {
	hs.Lock()
	defer hs.Unlock()
	hs.checkSuccessCache = val
}

func gtgCheck(handler func() (string, error)) gtg.StatusChecker {
	return func() gtg.Status {
		if _, err := handler(); err != nil {
			return gtg.Status{GoodToGo: false, Message: err.Error()}
		}
		return gtg.Status{GoodToGo: true}
	}
}
