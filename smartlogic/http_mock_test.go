package smartlogic

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

type mockHTTPClient struct {
	resp       string
	statusCode int
	err        error
}

func (c mockHTTPClient) Do(req *http.Request) (resp *http.Response, err error) {
	cb := ioutil.NopCloser(bytes.NewReader([]byte(c.resp)))
	return &http.Response{Body: cb, StatusCode: c.statusCode}, c.err
}
