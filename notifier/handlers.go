package notifier

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	transactionidutils "github.com/Financial-Times/transactionid-utils-go"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// TimeFormat is the format used to read time values from request parameters
const TimeFormat = "2006-01-02T15:04:05Z"

// maxTimeValue represents the maximum useful time value (for comparisons like finding the minimum value in a range of times)
var maxTimeValue = time.Unix(1<<63-62135596801, 999999999)

// LastChangeLimit represents the upper limit to how far in the past we can reingest smartlogic updates
var LastChangeLimit = time.Hour * 168

type Handler struct {
	notifier  Servicer
	ticker    Ticker
	requestCh chan notificationRequest
}

func NewNotifierHandler(notifier Servicer, opts ...func(*Handler)) *Handler {
	h := &Handler{
		notifier:  notifier,
		ticker:    &ticker{ticker: time.NewTicker(5 * time.Second)},
		requestCh: make(chan notificationRequest, 1),
	}

	for _, opt := range opts {
		opt(h)
	}

	go h.processNotifyRequests()

	return h
}

type Ticker interface {
	Tick()
	Stop()
}

func WithTicker(t Ticker) func(*Handler) {
	return func(h *Handler) {
		h.ticker.Stop()
		h.ticker = t
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

	lastChange, err := time.Parse(TimeFormat, lastChangeDate)
	if err != nil {
		writeResponseMessage(resp,
			http.StatusBadRequest,
			"application/json",
			fmt.Sprintf("{\"message\": \"Date is not in the format %s\"}", TimeFormat))
		return
	}
	log.WithField("time", lastChange).Debug("Parsing notification time")
	lastChange = lastChange.Add(-10 * time.Millisecond)
	log.WithField("time", lastChange).Debug("Subtracting notification time wobble")

	if time.Since(lastChange) > LastChangeLimit {
		writeJSONResponseMessage(resp, http.StatusBadRequest, fmt.Sprintf("Last change date should be time point in the last %.0f hours", LastChangeLimit.Hours()))
		return
	}

	go func() {
		h.requestCh <- notificationRequest{
			notifySince:   lastChange,
			transactionID: req.Header.Get(transactionidutils.TransactionIDHeader),
		}
	}()

	writeJSONResponseMessage(resp, http.StatusOK, "Concepts successfully ingested")
}

func (h *Handler) HandleGetConcepts(resp http.ResponseWriter, req *http.Request) {
	vars := req.URL.Query()
	lastChangeDate := vars.Get("lastChangeDate")
	if lastChangeDate == "" {
		writeJSONResponseMessage(resp, http.StatusBadRequest, `Query parameter lastChangeDate was not set.`)
		return
	}

	lastChange, err := time.Parse(TimeFormat, lastChangeDate)
	if err != nil {
		writeResponseMessage(resp,
			http.StatusBadRequest,
			"application/json",
			fmt.Sprintf("{\"message\": \"Date is not in the format %s\"}", TimeFormat))
		return
	}
	log.WithField("time", lastChange).Debug("Parsing notification time")
	lastChange = lastChange.Add(-10 * time.Millisecond)
	log.WithField("time", lastChange).Debug("Subtracting notification time wobble")

	if time.Since(lastChange) > LastChangeLimit {
		writeJSONResponseMessage(resp, http.StatusBadRequest, fmt.Sprintf("Last change date should be time point in the last %s", LastChangeLimit.String()))
		return
	}

	uuids, err := h.notifier.GetChangedConceptList(lastChange)
	if err != nil {
		writeResponseMessage(resp, http.StatusInternalServerError, "application/json", `{"message": "There was an error getting the changes", "error": "`+err.Error()+`"}`)
		return
	}
	uuidsJson, err := json.Marshal(uuids)
	if err != nil {
		writeResponseMessage(resp, http.StatusInternalServerError, "application/json", `{"message": "There was an error encoding the response", "error": "`+err.Error()+`"}`)
	}

	writeResponseMessage(resp, http.StatusOK, "application/json", string(uuidsJson))
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

	err = h.notifier.ForceNotify(pl.UUIDs, req.Header.Get(transactionidutils.TransactionIDHeader))
	if err != nil {
		writeJSONResponseMessage(resp, http.StatusInternalServerError, "There was an error completing the force notify")
		return
	}
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
	getConceptsHandler := handlers.MethodHandler{
		"GET": http.HandlerFunc(h.HandleGetConcepts),
	}

	router.Handle("/notify", notifyHandler)
	router.Handle("/force-notify", forceNotifyHandler)
	router.Handle("/concept/{uuid}", getConceptHandler)
	router.Handle("/concepts", getConceptsHandler)
}

type notificationRequest struct {
	notifySince   time.Time
	transactionID string
}

type ticker struct {
	ticker *time.Ticker
}

func (t *ticker) Tick() {
	<-t.ticker.C
}

func (t *ticker) Stop() {
	t.ticker.Stop()
}

func (h *Handler) processNotifyRequests() {
	for {
		h.ticker.Tick()

		if len(h.requestCh) == 0 {
			continue
		}

		n := notificationRequest{notifySince: maxTimeValue}
		for req := range h.requestCh {
			if n.notifySince.After(req.notifySince) {
				n = req
			}

			if len(h.requestCh) == 0 {
				break
			}
		}

		err := h.notifier.Notify(n.notifySince, n.transactionID)
		if err != nil {
			continue
		}
	}
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
