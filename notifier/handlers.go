package notifier

import (
	"encoding/json"
	"net/http"
	"strings"

	"time"

	transactionidutils "github.com/Financial-Times/transactionid-utils-go"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var maxTimeValue = time.Unix(1<<63-62135596801, 999999999)

type Handler struct {
	notifier  Servicer
	ticker    *time.Ticker
	requestCh chan notificationRequest
}

func NewNotifierHandler(notifier Servicer, opts ...func(*Handler)) *Handler {
	h := &Handler{
		notifier:  notifier,
		ticker:    time.NewTicker(5 * time.Second),
		requestCh: make(chan notificationRequest, 1),
	}

	for _, opt := range opts {
		opt(h)
	}

	go h.processNotifyRequests()

	return h
}

func WithTickerInterval(d time.Duration) func(*Handler) {
	return func(h *Handler) {
		h.ticker.Stop()
		h.ticker = time.NewTicker(d)
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

	go func() {
		h.requestCh <- notificationRequest{
			notifySince:   lastChange,
			transactionID: req.Header.Get(transactionidutils.TransactionIDHeader),
		}
	}()

	writeJSONResponseMessage(resp, http.StatusOK, "Concepts successfully ingested")
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

	router.Handle("/notify", notifyHandler)
	router.Handle("/force-notify", forceNotifyHandler)
	router.Handle("/concept/{uuid}", getConceptHandler)
}

type notificationRequest struct {
	notifySince   time.Time
	transactionID string
}

func (h *Handler) processNotifyRequests() {
	for {
		<-h.ticker.C

		log.Info("tick")

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
			return
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
