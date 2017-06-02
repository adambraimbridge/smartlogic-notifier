package notifier

import (
	"net/http"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/http-handlers-go/httphandlers"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/Financial-Times/smartlogic-notifier/health"
	"github.com/Financial-Times/smartlogic-notifier/kafka"
	"github.com/Financial-Times/smartlogic-notifier/smartlogic"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	metrics "github.com/rcrowley/go-metrics"
)

type Handler struct {
	kafka      kafka.Client
	smartlogic smartlogic.Client
}

func NewNotifierHandler(kafka kafka.Client, smartlogic smartlogic.Client) *Handler {
	return &Handler{
		kafka:      kafka,
		smartlogic: smartlogic,
	}
}

func (h *Handler) HandleNotify(resp http.ResponseWriter, req *http.Request) {
	//vars := mux.Vars(req)

}

func (h *Handler) HandleForceNotify(resp http.ResponseWriter, req *http.Request) {
	//vars := mux.Vars(req)

}

func (h *Handler) HandleGetConcept(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	uuid, ok := vars["uuid"]
	if !ok {
		// handle 400
	}

	concept, err := h.smartlogic.GetConcept(uuid)
	if err != nil {
		// handle 500
	}
	resp.Header().Set("Content-Type", "application/ld+json")
	resp.Write(concept)
}

func (h *Handler) RegisterEndpoints(router *mux.Router) {
	notifyHandler := handlers.MethodHandler{
		"GET": http.HandlerFunc(h.HandleNotify),
	}
	forceNotifyHandler := handlers.MethodHandler{
		"POST": http.HandlerFunc(h.HandleForceNotify),
	}
	getConceptHandler := handlers.MethodHandler{
		"GET": http.HandlerFunc(h.HandleGetConcept),
	}

	router.Handle("/notify", notifyHandler)
	router.Handle("/force-notify", forceNotifyHandler)
	router.Handle("/concept/{uuid}", getConceptHandler)
}

func (h *Handler) RegisterAdminEndpoints(router *mux.Router, appSystemCode string, appName string, description string) {
	healthService := health.NewHealthService(appSystemCode, appName, description)

	hc := fthealth.HealthCheck{SystemCode: appSystemCode, Name: appName, Description: description, Checks: healthService.Checks}
	router.HandleFunc("/__health", fthealth.Handler(hc))
	router.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.GtgCheck))
	router.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)

	var monitoringRouter http.Handler = router
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(log.StandardLogger(), monitoringRouter)
	monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)
}
