package notifier

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func TestHandlers(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	testCases := []struct {
		name        string
		method      string
		url         string
		requestBody string
		resultCode  int
		resultBody  string
		mockService *MockService
	}{
		{
			"Notify - Success",
			"GET",
			"/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=2017-05-31T13:00:00.000Z",
			"",
			200,
			"{\"message\": \"Messages successfully sent to Kafka\"}",
			&MockService{
				notify: func(i time.Time, s string) error {
					return nil
				},
			},
		},
		{
			"Notify - Missing query parameters",
			"GET",
			"/notify?modifiedGraphId=2&lastChangeDate=2017-05-31T13:00:00.000Z",
			"",
			400,
			"{\"message\": \"Query parameters were not set: affectedGraphId\"}",
			&MockService{},
		},
		{
			"Notify - Missing all query parameters",
			"GET",
			"/notify",
			"",
			400,
			"{\"message\": \"Query parameters were not set: modifiedGraphId, affectedGraphId, lastChangeDate\"}",
			&MockService{},
		},
		{
			"Notify - Bad query parameters ",
			"GET",
			"/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=notadate",
			"",
			400,
			"{\"message\": \"Date is not in the format 2006-01-02T15:04:05.000Z\"}",
			&MockService{},
		},
		{
			"Notify - Error",
			"GET",
			"/notify?affectedGraphId=1&modifiedGraphId=2&lastChangeDate=2017-05-31T13:00:00.000Z",
			"",
			500,
			"{\"message\": \"There was an error completing the notify\", \"error\": \"anerror\"}",
			&MockService{
				notify: func(i time.Time, s string) error {
					return errors.New("anerror")
				},
			},
		},
		{
			"Force Notify - Success",
			"POST",
			"/force-notify",
			`{"uuids": ["1","2","3"]}`,
			200,
			"Concept notification completed",
			&MockService{},
		},
		{
			"Force Notify - Bad Payload",
			"POST",
			"/force-notify",
			`{"uuids": "1","2","3"]}`,
			400,
			"{\"message\": \"There was an error decoding the payload\", \"error\": \"invalid character ',' after object key\"}",
			&MockService{},
		},
		{
			"Get Concept - Success",
			"GET",
			"/concept/1",
			``,
			200,
			"1",
			&MockService{
				getConcept: func(s string) ([]byte, error) {
					return []byte("1"), nil
				},
			},
		},
		{
			"Get Concept - Error",
			"GET",
			"/concept/11",
			``,
			500,
			"{\"message\": \"There was an error retrieving the concept\", \"error\": \"can't find concept\"}",
			&MockService{
				getConcept: func(s string) ([]byte, error) {
					return nil, errors.New("can't find concept")
				},
			},
		},
		{
			"__health",
			"GET",
			"/__health",
			``,
			200,
			"IGNORE",
			&MockService{},
		},
		{
			"__build-info",
			"GET",
			"/__build-info",
			``,
			200,
			"IGNORE",
			&MockService{},
		},
		{
			"__gtg",
			"GET",
			"/__gtg",
			``,
			503,
			"IGNORE",
			&MockService{},
		},
	}

	for _, d := range testCases {
		t.Run(d.name, func(t *testing.T) {
			handler := NewNotifierHandler(d.mockService)
			m := mux.NewRouter()
			handler.RegisterEndpoints(m)
			handler.RegisterAdminEndpoints(m, "system-code", "app-name", "description", "testModel", time.Minute)

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

func TestHealthCheckError(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	mockSvc := &MockService{
		getConcept: func(s string) ([]byte, error) {
			return nil, errors.New("some error")
		},
	}
	handler := NewNotifierHandler(mockSvc)
	m := mux.NewRouter()
	handler.RegisterEndpoints(m)
	handler.RegisterAdminEndpoints(m, "system-code", "app-name", "description", "testModel", time.Second)

	req, _ := http.NewRequest("GET", "/__gtg", bytes.NewBufferString(""))
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, req)

	b, err := ioutil.ReadAll(rr.Body)
	assert.NoError(t, err)
	body := string(b)
	assert.Equal(t, 503, rr.Code, "__gtg")
	assert.Equal(t, "latest Smartlogic connectivity check is unsuccessful", body, "__gtg")
}

func TestHealthServiceChecks(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	tests := []struct {
		name           string
		url            string
		mockService    *MockService
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "gtg endpoint success",
			url:  "/__gtg",
			mockService: &MockService{
				getConcept: func(s string) ([]byte, error) {
					return []byte(""), nil
				},
			},
			expectedStatus: 200,
			expectedBody:   "OK",
		},
		{
			name: "health endpoint success",
			url:  "/__health",
			mockService: &MockService{
				getConcept: func(s string) ([]byte, error) {
					return []byte(""), nil
				},
			},
			expectedStatus: 200,
			expectedBody:   `"ok":true`,
		},
		{
			name: "gtg endpoint failure",
			url:  "/__gtg",
			mockService: &MockService{
				getConcept: func(s string) ([]byte, error) {
					return nil, errors.New("couldn't retrieve FT organisation from Smartlogic")
				},
			},
			expectedStatus: 503,
			expectedBody:   "latest Smartlogic connectivity check is unsuccessful",
		},
		{
			name: "health endpoint failure",
			url:  "/__health",
			mockService: &MockService{
				getConcept: func(s string) ([]byte, error) {
					return nil, errors.New("couldn't retrieve FT organisation from Smartlogic")
				},
			},
			expectedStatus: 200, // the __health endpoint always returns 200
			expectedBody:   `"ok":false`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewNotifierHandler(test.mockService)
			m := mux.NewRouter()
			handler.RegisterEndpoints(m)
			handler.RegisterAdminEndpoints(m, "system-code", "app-name", "description", "testModel", time.Second)

			// give time the cache of the Healthcheck service to be updated to be updated (getConcept to be called)
			time.Sleep(time.Second)

			req, err := http.NewRequest("GET", test.url, bytes.NewBufferString(""))
			if err != nil {
				t.Fatalf("couldn't call %s", test.url)
			}
			rr := httptest.NewRecorder()
			m.ServeHTTP(rr, req)

			b, err := ioutil.ReadAll(rr.Body)
			assert.NoError(t, err)
			body := string(b)
			assert.Equal(t, test.expectedStatus, rr.Code, test.url)
			assert.Contains(t, body, test.expectedBody, test.url)
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
			name:                  "Test the cache for __heathcheck endpoint",
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
			okay := true
			okayMu := sync.RWMutex{}
			getConcept := func(s string) ([]byte, error) {
				okayMu.Lock()
				defer okayMu.Unlock()
				if okay {
					return []byte(""), nil
				}
				return nil, errors.New("FT concept couldn't be retrieved")
			}

			mockSvc := &MockService{
				getConcept: getConcept,
			}
			handler := NewNotifierHandler(mockSvc)
			m := mux.NewRouter()
			handler.RegisterEndpoints(m)
			handler.RegisterAdminEndpoints(m, "system-code", "app-name", "description", "testModel", 2*time.Second)
			// give time the cache of the Healthcheck service to be updated to be updated (getConcept to be called)
			time.Sleep(1*time.Second)

			// check we return ok
			{
				req, _ := http.NewRequest("GET", "/"+test.url, bytes.NewBufferString(""))
				rr := httptest.NewRecorder()
				m.ServeHTTP(rr, req)
				b, err := ioutil.ReadAll(rr.Body)
				assert.NoError(t, err)
				body := string(b)
				assert.Equal(t, 200, rr.Code, test.url)
				assert.Contains(t, body, test.expectedSuccessBody, test.url)
			}

			// tell GetConcept to return error, mocking we couldn't get the FT concept from Smartlogic
			okayMu.Lock()
			okay = false
			okayMu.Unlock()

			// but expect to return cached ok
			{
				req, _ := http.NewRequest("GET", "/"+test.url, bytes.NewBufferString(""))
				rr := httptest.NewRecorder()
				m.ServeHTTP(rr, req)
				b, err := ioutil.ReadAll(rr.Body)
				assert.NoError(t, err)
				body := string(b)
				assert.Equal(t, 200, rr.Code, test.url)
				assert.Contains(t, body, test.expectedSuccessBody, test.url)
			}

			// wait for cache to clear
			time.Sleep(3 * time.Second)

			// and expect gtg to return err
			{
				req, _ := http.NewRequest("GET", "/"+test.url, bytes.NewBufferString(""))
				rr := httptest.NewRecorder()
				m.ServeHTTP(rr, req)
				b, err := ioutil.ReadAll(rr.Body)
				assert.NoError(t, err)
				body := string(b)
				assert.Equal(t, test.expectedFailureStatus, rr.Code, test.url)
				assert.Contains(t, body, test.expectedFailureBody, test.url)
			}

			// tell GetConcept to return okay
			okayMu.Lock()
			okay = true
			okayMu.Unlock()
			// wait for cache to clear
			time.Sleep(3 * time.Second)

			// and expect gtg to return ok
			{
				req, _ := http.NewRequest("GET", "/"+test.url, bytes.NewBufferString(""))
				rr := httptest.NewRecorder()
				m.ServeHTTP(rr, req)
				b, err := ioutil.ReadAll(rr.Body)
				assert.NoError(t, err)
				body := string(b)
				assert.Equal(t, 200, rr.Code, test.url)
				assert.Contains(t, body, test.expectedSuccessBody, test.url)
			}
		})
	}
}
