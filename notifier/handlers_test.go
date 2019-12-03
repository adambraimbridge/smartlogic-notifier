package notifier

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	http "net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestHandlers(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	today := time.Now().Format(TimeFormat)
	past := time.Date(1900, 1, 1, 0, 0, 0, 0, time.Local).Format(TimeFormat)
	testCases := []struct {
		name        string
		method      string
		url         string
		requestBody string
		resultCode  int
		resultBody  string
		mockService *mockService
	}{
		{
			name:       "Notify - Success",
			method:     "GET",
			url:        fmt.Sprintf("/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=%s", today),
			resultBody: "{\"message\": \"Concepts successfully ingested\"}",
			resultCode: 200,
			mockService: &mockService{
				notify: func(i time.Time, s string) error {
					return nil
				},
			},
		},
		{
			name:        "Notify - Missing query parameters",
			method:      "GET",
			url:         "/notify?modifiedGraphId=2&lastChangeDate=2017-05-31T13:00:00.000Z",
			resultCode:  400,
			resultBody:  "{\"message\": \"Query parameters were not set: affectedGraphId\"}",
			mockService: &mockService{},
		},
		{
			name:        "Notify - Missing all query parameters",
			method:      "GET",
			url:         "/notify",
			resultCode:  400,
			resultBody:  "{\"message\": \"Query parameters were not set: modifiedGraphId, affectedGraphId, lastChangeDate\"}",
			mockService: &mockService{},
		},
		{
			name:        "Notify - Bad query parameters ",
			method:      "GET",
			url:         "/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=notadate",
			resultCode:  400,
			resultBody:  "{\"message\": \"Date is not in the format 2006-01-02T15:04:05Z\"}",
			mockService: &mockService{},
		},
		{
			name:        "Notify - Bad lastChangeDate parameter",
			method:      "GET",
			url:         fmt.Sprintf("/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=%s", past),
			resultCode:  400,
			resultBody:  fmt.Sprintf("{\"message\": \"Last change date should be time point in the last %.0f hours\"}", LastChangeLimit.Hours()),
			mockService: &mockService{},
		},
		{
			name:       "Notify - Error",
			method:     "GET",
			url:        fmt.Sprintf("/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=%s", today),
			resultBody: "{\"message\": \"Concepts successfully ingested\"}",
			resultCode: 200,
			mockService: &mockService{
				notify: func(i time.Time, s string) error {
					return errors.New("anerror")
				},
			},
		},
		{
			name:        "Force Notify - Success",
			method:      "POST",
			url:         "/force-notify",
			requestBody: `{"uuids": ["1","2","3"]}`,
			resultCode:  200,
			resultBody:  "Concept notification completed",
			mockService: &mockService{
				forceNotify: func(strings []string, s string) error {
					return nil
				},
			},
		},
		{
			name:        "Force Notify - Bad Payload",
			method:      "POST",
			url:         "/force-notify",
			requestBody: `{"uuids": "1","2","3"]}`,
			resultCode:  400,
			resultBody:  "{\"message\": \"There was an error decoding the payload\", \"error\": \"invalid character ',' after object key\"}",
			mockService: &mockService{},
		},
		{
			name:        "Force Notify - Failure",
			method:      "POST",
			url:         "/force-notify",
			requestBody: `{"uuids": ["1","2","3"]}`,
			resultCode:  500,
			resultBody:  "{\"message\": \"There was an error completing the force notify\"}",
			mockService: &mockService{
				forceNotify: func(strings []string, s string) error {
					return errors.New("error in force notify")
				},
			},
		},
		{
			name:       "Get Concept - Success",
			method:     "GET",
			url:        "/concept/1",
			resultCode: 200,
			resultBody: "1",
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return []byte("1"), nil
				},
			},
		},
		{
			name:       "Get Concept - Error",
			method:     "GET",
			url:        "/concept/11",
			resultCode: 500,
			resultBody: "{\"message\": \"There was an error retrieving the concept\", \"error\": \"can't find concept\"}",
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return nil, errors.New("can't find concept")
				},
			},
		},
		{
			name:       "Get Concepts - Success",
			method:     "GET",
			url:        fmt.Sprintf("/concepts?lastChangeDate=%s", today),
			resultCode: 200,
			resultBody: `["1","2","3"]`,
			mockService: &mockService{
				getConcepts: func(t time.Time) ([]string, error) {
					return []string{"1", "2", "3"}, nil
				},
			},
		},
		{
			name:       "Get Concepts - Invalid Time",
			method:     "GET",
			url:        fmt.Sprintf("/concepts?lastChangeDate=%s", past),
			resultCode: 400,
			resultBody: fmt.Sprintf("{\"message\": \"Last change date should be time point in the last %.0f hours\"}", LastChangeLimit.Hours()),
			mockService: &mockService{
				getConcepts: func(t time.Time) ([]string, error) {
					return []string{"1", "2", "3"}, nil
				},
			},
		},
		{
			name:       "Get Concepts - Bad Time format",
			method:     "GET",
			url:        "/concepts?lastChangeDate=nodata",
			resultCode: 400,
			resultBody: "{\"message\": \"Date is not in the format 2006-01-02T15:04:05Z\"}",
			mockService: &mockService{
				getConcepts: func(t time.Time) ([]string, error) {
					return []string{"1", "2", "3"}, nil
				},
			},
		},
		{
			name:       "Get Concepts - No Time",
			method:     "GET",
			url:        "/concepts",
			resultCode: 400,
			resultBody: "{\"message\": \"Query parameter lastChangeDate was not set.\"}",
			mockService: &mockService{
				getConcepts: func(t time.Time) ([]string, error) {
					return []string{"1", "2", "3"}, nil
				},
			},
		},
		{
			name:       "Get Concepts - Smartlogic error",
			method:     "GET",
			url:        fmt.Sprintf("/concepts?lastChangeDate=%s", today),
			resultCode: 500,
			resultBody: "{\"message\": \"There was an error getting the changes\", \"error\": \"smartlogic error\"}",
			mockService: &mockService{
				getConcepts: func(t time.Time) ([]string, error) {
					return nil, errors.New("smartlogic error")
				},
			},
		},
		{
			name:        "__health",
			method:      "GET",
			url:         "/__health",
			resultCode:  200,
			resultBody:  "IGNORE",
			mockService: &mockService{},
		},
		{
			name:        "__build-info",
			method:      "GET",
			url:         "/__build-info",
			resultCode:  200,
			resultBody:  "IGNORE",
			mockService: &mockService{},
		},
		{
			name:        "__gtg",
			method:      "GET",
			url:         "/__gtg",
			resultCode:  503,
			resultBody:  "IGNORE",
			mockService: &mockService{},
		},
	}

	for _, d := range testCases {
		t.Run(d.name, func(t *testing.T) {
			handler := NewNotifierHandler(d.mockService)
			m := mux.NewRouter()
			handler.RegisterEndpoints(m)

			healthService := NewHealthService(d.mockService, "system-code", "app-name", "description", "testModel", time.Minute)
			healthService.Start()
			_ = healthService.RegisterAdminEndpoints(m)

			req, _ := http.NewRequest(d.method, d.url, bytes.NewBufferString(d.requestBody))
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)

			b, err := ioutil.ReadAll(rr.Body)
			assert.NoError(t, err)
			body := string(b)
			assert.Equal(t, d.resultCode, rr.Code, d.name)
			if d.resultBody != "IGNORE" {
				assert.Equal(t, d.resultBody, body, d.name)
			}

		})
	}

}

func TestConcurrentNotify(t *testing.T) {
	testCases := []struct {
		name                           string
		notificationTimes              []string
		duration                       time.Duration
		expectedChangedConceptListDate string
		expectedKafkaMsgCount          int
	}{
		{
			name: "single request",
			notificationTimes: []string{
				"13:00:04.000Z",
			},
			duration:                       50 * time.Millisecond,
			expectedChangedConceptListDate: "13:00:03.990Z",
			expectedKafkaMsgCount:          1,
		},
		{
			name: "10 requests",
			notificationTimes: []string{
				"13:00:00.000Z",
				"13:00:01.000Z",
				"13:00:02.000Z",
				"13:00:03.000Z",
				"13:00:04.000Z",
				"13:00:05.000Z",
				"13:00:06.000Z",
				"13:00:07.000Z",
				"13:00:08.000Z",
				"13:00:09.000Z",
			},
			expectedChangedConceptListDate: "12:59:59.990Z",
			duration:                       50 * time.Millisecond,
			expectedKafkaMsgCount:          1,
		},
	}

	today := time.Now().Format("2006-01-02T")
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			kc := &mockKafkaClient{}
			sl := &mockSmartlogicClient{
				concepts: map[string]string{
					"uuid1": "concept1",
					"uuid2": "concept2",
				},
				getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
					assert.Equal(t, test.expectedChangedConceptListDate, changeDate.Format("15:04:05.000Z"))

					return []string{"uuid2"}, nil
				},
			}

			service := NewNotifierService(kc, sl)

			// making sure the ticker will fire at least once during the test execution
			tickerInterval := test.duration / 2
			tk := &ticker{ticker: time.NewTicker(tickerInterval)}

			handler := NewNotifierHandler(service, WithTicker(tk))

			m := mux.NewRouter()
			handler.RegisterEndpoints(m)

			var reqs []*http.Request
			for _, tPoint := range test.notificationTimes {
				url := fmt.Sprintf("/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=%s%s", today, tPoint)
				req, _ := http.NewRequest("GET", url, nil)
				reqs = append(reqs, req)
			}

			for _, req := range reqs {
				go func(r *http.Request) {
					start := time.Now()
					recorder := httptest.NewRecorder()
					m.ServeHTTP(recorder, r)
					end := time.Now()

					assert.WithinDuration(t, start, end, tickerInterval)
				}(req)
			}

			time.Sleep(test.duration)

			assert.Equal(t, test.expectedKafkaMsgCount, kc.getSentCount())
		})
	}
}

func TestProcessingNotifyRequestsDoesNotBlock(t *testing.T) {
	testCases := []struct {
		name              string
		notificationTimes []string
		duration          time.Duration
		slClient          *mockSmartlogicClient
		expectedTicks     int
	}{
		{
			name: "no requests",
			slClient: &mockSmartlogicClient{
				concepts: map[string]string{
					"uuid1": "concept1",
					"uuid2": "concept2",
				},
				getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
					return []string{"uuid2"}, nil
				},
			},
			duration:      100 * time.Millisecond,
			expectedTicks: 10,
		},
		{
			name: "single request",
			slClient: &mockSmartlogicClient{
				concepts: map[string]string{
					"uuid1": "concept1",
					"uuid2": "concept2",
				},
				getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
					return []string{"uuid2"}, nil
				},
			},
			notificationTimes: []string{
				"13:00:04.000Z",
			},
			duration:      100 * time.Millisecond,
			expectedTicks: 10,
		},
		{
			name: "10 requests",
			slClient: &mockSmartlogicClient{
				concepts: map[string]string{
					"uuid1": "concept1",
					"uuid2": "concept2",
				},
				getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
					return []string{"uuid2"}, nil
				},
			},
			notificationTimes: []string{
				"13:00:00.000Z",
				"13:00:01.000Z",
				"13:00:02.000Z",
				"13:00:03.000Z",
				"13:00:04.000Z",
				"13:00:05.000Z",
				"13:00:06.000Z",
				"13:00:07.000Z",
				"13:00:08.000Z",
				"13:00:09.000Z",
			},
			duration:      1000 * time.Millisecond,
			expectedTicks: 10,
		},
		{
			name: "5 requests with errors on get changed concepts list",
			slClient: &mockSmartlogicClient{
				concepts: nil,
				getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
					return nil, errors.New("could not get changed concepts list")
				},
			},
			notificationTimes: []string{
				"13:00:03.000Z",
				"13:00:04.000Z",
				"13:00:05.000Z",
				"13:00:06.000Z",
				"13:00:07.000Z",
			},
			duration:      1000 * time.Millisecond,
			expectedTicks: 10,
		},
	}

	today := time.Now().Format("2017-05-31T")
	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			kc := &mockKafkaClient{}
			service := NewNotifierService(kc, test.slClient)

			tickerInterval := test.duration / 10
			tk := &mockTicker{ticker: time.NewTicker(tickerInterval)}

			handler := NewNotifierHandler(service, WithTicker(tk))

			m := mux.NewRouter()
			handler.RegisterEndpoints(m)

			var reqs []*http.Request
			for _, tPoint := range test.notificationTimes {
				url := fmt.Sprintf("/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=%s%s", today, tPoint)
				req, _ := http.NewRequest("GET", url, nil)
				reqs = append(reqs, req)
			}

			for _, req := range reqs {
				go func(r *http.Request) {
					start := time.Now()
					recorder := httptest.NewRecorder()
					m.ServeHTTP(recorder, r)
					end := time.Now()

					assert.WithinDuration(t, start, end, tickerInterval)
				}(req)
			}

			time.Sleep(test.duration)

			assert.InDelta(t, test.expectedTicks, tk.getTicks(), 1)
		})
	}
}

func TestGettingSmartlogicChangesOneRequestAtATime(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	testCases := []struct {
		name     string
		reqCount int
		duration time.Duration
	}{
		{
			name:     "100 notify requests",
			reqCount: 100,
			duration: 500 * time.Millisecond,
		},
	}
	today := time.Now().Format(TimeFormat)
	requestURI := fmt.Sprintf("/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=%s", today)
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			kc := &mockKafkaClient{}
			sl := &mockSmartlogicClient{
				concepts: map[string]string{
					"uuid1": "concept1",
					"uuid2": "concept2",
				},
				getChangedConceptListFunc: func(changeDate time.Time) ([]string, error) {
					return []string{"uuid2"}, nil
				},
			}

			service := NewNotifierService(kc, sl)

			tickerInterval := test.duration / 5
			tk := &ticker{ticker: time.NewTicker(tickerInterval)}

			handler := NewNotifierHandler(service, WithTicker(tk))

			m := mux.NewRouter()
			handler.RegisterEndpoints(m)

			for i := 0; i < test.reqCount; i++ {
				go func() {
					r, _ := http.NewRequest("GET", requestURI, nil)
					start := time.Now()
					recorder := httptest.NewRecorder()
					m.ServeHTTP(recorder, r)
					end := time.Now()

					assert.WithinDuration(t, start, end, tickerInterval)
				}()
			}

			time.Sleep(test.duration)

			assert.Equal(t, 1, sl.getChangedConceptListCallCount())
		})
	}
}

func TestHealthServiceChecks(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	tests := []struct {
		name           string
		url            string
		mockService    *mockService
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "gtg endpoint success",
			url:  "__gtg",
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return []byte(""), nil
				},
				checkKafkaConnectivity: func() error {
					return nil
				},
			},
			expectedStatus: 200,
			expectedBody:   "OK",
		},
		{
			name: "health endpoint success",
			url:  "__health",
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return []byte(""), nil
				},
				checkKafkaConnectivity: func() error {
					return nil
				},
			},
			expectedStatus: 200,
			expectedBody:   `"ok":true}`,
		},
		{
			name: "gtg endpoint Smartlogic failure",
			url:  "__gtg",
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return nil, errors.New("couldn't retrieve FT organisation from Smartlogic")
				},
				checkKafkaConnectivity: func() error {
					return nil
				},
			},
			expectedStatus: 503,
			expectedBody:   "latest Smartlogic connectivity check is unsuccessful",
		},
		{
			name: "gtg endpoint Kafka failure",
			url:  "__gtg",
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return []byte(""), nil
				},
			},
			expectedStatus: 503,
			expectedBody:   "Error verifying open connection to Kafka",
		},
		{
			name: "health endpoint Smartlogic failure",
			url:  "__health",
			mockService: &mockService{
				checkKafkaConnectivity: func() error {
					return nil
				},
			},
			expectedStatus: 200, // the __health endpoint always returns 200
			expectedBody:   `"ok":false,"severity":3}`,
		},
		{
			name: "health endpoint Kafka failure",
			url:  "__health",
			mockService: &mockService{
				getConcept: func(s string) ([]byte, error) {
					return []byte(""), nil
				},
			},
			expectedStatus: 200, // the __health endpoint always returns 200
			expectedBody:   `"ok":false,"severity":3}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := mux.NewRouter()
			healthcheckCacheInterval := 10 * time.Millisecond
			healthService := NewHealthService(test.mockService, "system-code", "app-name", "description", "testModel", healthcheckCacheInterval)
			healthService.Start()
			_ = healthService.RegisterAdminEndpoints(m)

			// give time the cache of the Healthcheck service to be updated to be updated (getConcept to be called)
			time.Sleep(healthcheckCacheInterval)

			assertRequest(t, m, test.url, test.expectedBody, test.expectedStatus)
		})
	}
}

func TestHealthServiceCache(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	tests := []struct {
		name                  string
		url                   string
		expectedFailureStatus int
		expectedSuccessBody   string
		expectedFailureBody   string
	}{
		{
			name:                  "Test the cache for __health endpoint",
			url:                   "__health",
			expectedFailureStatus: 200,
			expectedSuccessBody:   `"ok":true`,
			expectedFailureBody:   `"ok":false`,
		},
		{
			name:                  "Test the cache for __gtg endpoint",
			url:                   "__gtg",
			expectedFailureStatus: 503,
			expectedSuccessBody:   "OK",
			expectedFailureBody:   "latest Smartlogic connectivity check is unsuccessful",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ok := true
			okMutex := sync.RWMutex{}
			getConcept := func(s string) ([]byte, error) {
				okMutex.Lock()
				defer okMutex.Unlock()
				if ok {
					return []byte(""), nil
				}
				return nil, errors.New("FT concept couldn't be retrieved")
			}

			mockSvc := &mockService{
				getConcept: getConcept,
				checkKafkaConnectivity: func() error {
					return nil
				},
			}
			m := mux.NewRouter()

			healthcheckCacheInterval := 20 * time.Millisecond
			healthService := NewHealthService(mockSvc, "system-code", "app-name", "description", "testModel", healthcheckCacheInterval)
			healthService.Start()
			_ = healthService.RegisterAdminEndpoints(m)
			// give time the cache of the Healthcheck service to be updated to be updated (getConcept to be called)
			time.Sleep(healthcheckCacheInterval / 2)

			// check we return ok
			assertRequest(t, m, test.url, test.expectedSuccessBody, 200)

			// tell GetConcept to return error, mocking we couldn't get the FT concept from Smartlogic
			okMutex.Lock()
			ok = false
			okMutex.Unlock()

			// but expect to return cached ok
			assertRequest(t, m, test.url, test.expectedSuccessBody, 200)

			// wait for cache to clear
			time.Sleep(3 * healthcheckCacheInterval / 2)

			// and expect gtg to return err
			assertRequest(t, m, test.url, test.expectedFailureBody, test.expectedFailureStatus)

			// tell GetConcept to return okay
			okMutex.Lock()
			ok = true
			okMutex.Unlock()
			// wait for cache to clear
			time.Sleep(3 * healthcheckCacheInterval / 2)

			// and expect gtg to return ok
			assertRequest(t, m, test.url, test.expectedSuccessBody, 200)
		})
	}
}

func assertRequest(t *testing.T, m http.Handler, url string, expectedBody string, expectedStatus int) {
	req, err := http.NewRequest("GET", "/"+url, bytes.NewBufferString(""))
	if err != nil {
		t.Fatalf("failed creating new test requst to %s", url)
	}
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)
	b, err := ioutil.ReadAll(rr.Body)
	assert.NoError(t, err)
	body := string(b)
	assert.Equal(t, expectedStatus, rr.Code, url)
	assert.Contains(t, body, expectedBody, url)
}
