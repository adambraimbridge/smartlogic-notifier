package smartlogic

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

type mockHttpClient struct {
	resp       string
	statusCode int
	err        error
}

func (c mockHttpClient) Do(req *http.Request) (resp *http.Response, err error) {
	cb := ioutil.NopCloser(bytes.NewReader([]byte(c.resp)))
	return &http.Response{Body: cb, StatusCode: c.statusCode}, c.err
}
