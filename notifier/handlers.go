package notifier

import (
	"encoding/json"
	"net/http"
	"strings"

	"time"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/http-handlers-go/httphandlers"
	status "github.com/Financial-Times/service-status-go/httphandlers"
	"github.com/Financial-Times/transactionid-utils-go"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/rcrowley/go-metrics"
)

type Handler struct {
	notifier Servicer
}

func NewNotifierHandler(notifier Servicer) *Handler {
	return &Handler{
		notifier: notifier,
	}
}

func (h *Handler) HandleNotify(resp http.ResponseWriter, req *http.Request) {
	vars := req.URL.Query()
	var notSet []string
	modifiedGraphId := vars.Get("modifiedGraphId")
	if modifiedGraphId == "" {
		notSet = append(notSet, "modifiedGraphId")
	}
	affectedGraphId := vars.Get("affectedGraphId")
	if affectedGraphId == "" {
		notSet = append(notSet, "affectedGraphId")
	}
	lastChangeDate := vars.Get("lastChangeDate")
	if lastChangeDate == "" {
		notSet = append(notSet, "lastChangeDate")
	}

	if len(notSet) > 0 {
		writeJSONResponseMessage(resp, http.StatusBadRequest, `Query parameters were not set: `+strings.Join(notSet, ", "))
		return
	}

	lastChange, err := time.Parse("2006-01-02T15:04:05Z", lastChangeDate)
	log.WithField("time", lastChange).Debug("Parsing notification time")
	lastChange = lastChange.Add(-10 * time.Millisecond)
	log.WithField("time", lastChange).Debug("Subtracting notification time wobble")
	if err != nil {
		writeResponseMessage(resp, http.StatusBadRequest, "application/json", `{"message": "Date is not in the format 2006-01-02T15:04:05.000Z"}`)
		return
	}
	err = h.notifier.Notify(lastChange, req.Header.Get(transactionidutils.TransactionIDHeader))
	if err != nil {
		writeResponseMessage(resp, http.StatusInternalServerError, "application/json", `{"message": "There was an error completing the notify", "error": "`+err.Error()+`"}`)
		return
	}

	writeJSONResponseMessage(resp, http.StatusOK, "Messages successfully sent to Kafka")
}

func (h *Handler) HandleForceNotify(resp http.ResponseWriter, req *http.Request) {
	type payload struct {
		UUIDs []string `json:"uuids,omitempty"`
	}
	var pl payload
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&pl)
	if err != nil {
		writeResponseMessage(resp, http.StatusBadRequest, "application/json", `{"message": "There was an error decoding the payload", "error": "`+err.Error()+`"}`)
		return
	}

	if pl.UUIDs == nil {
		writeJSONResponseMessage(resp, http.StatusBadRequest, "No 'uuids' parameter provided")
		return
	}

	h.notifier.ForceNotify(pl.UUIDs, req.Header.Get(transactionidutils.TransactionIDHeader))
	writeResponseMessage(resp, http.StatusOK, "text/plain", "Concept notification completed")
}

func (h *Handler) HandleGetConcept(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	uuid, ok := vars["uuid"]

	if !ok {
		writeJSONResponseMessage(resp, http.StatusBadRequest, "UUID was not set.")
		return
	}

	concept, err := h.notifier.GetConcept(uuid)
	if err != nil {
		writeResponseMessage(resp, http.StatusInternalServerError, "application/json", `{"message": "There was an error retrieving the concept", "error": "`+err.Error()+`"}`)
		return
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

func (h *Handler) RegisterAdminEndpoints(router *mux.Router, appSystemCode string, appName string, description string) http.Handler {
	healthService := NewHealthService(h.notifier, appSystemCode, appName, description)

	hc := fthealth.HealthCheck{SystemCode: appSystemCode, Name: appName, Description: description, Checks: healthService.Checks}
	router.HandleFunc("/__health", fthealth.Handler(hc))
	router.HandleFunc(status.GTGPath, status.NewGoodToGoHandler(healthService.GtgCheck))
	router.HandleFunc(status.BuildInfoPath, status.BuildInfoHandler)

	var monitoringRouter http.Handler = router
	monitoringRouter = httphandlers.TransactionAwareRequestLoggingHandler(log.StandardLogger(), monitoringRouter)
	monitoringRouter = httphandlers.HTTPMetricsHandler(metrics.DefaultRegistry, monitoringRouter)

	return monitoringRouter
}

func writeResponseMessage(w http.ResponseWriter, statusCode int, contentType string, message string) {
	log.WithField("message", message).Debug("Creating response message")
	if contentType == "" {
		contentType = "text/plain"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(statusCode)
	w.Write([]byte(message))
}

func writeJSONResponseMessage(w http.ResponseWriter, statusCode int, message string) {
	msg := `{"message": "` + message + `"}`
	writeResponseMessage(w, statusCode, "application/json", msg)
}