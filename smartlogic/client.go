package smartlogic

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"

	log "github.com/Sirupsen/logrus"
)

const propertiesQueryParamValue = "[],skosxl:prefLabel/skosxl:literalForm"
const slTokenURL = "https://cloud.smartlogic.com/token"
const maxAccessFailureCount = 5

type httpClient interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

type Client struct {
	baseURL            url.URL
	model              string
	apiKey             string
	httpClient         httpClient
	accessToken        string
	accessFailureCount int
}

func NewSmartlogicClient(baseURL string, model string, apiKey string) (Client, error) {
	u, _ := url.Parse(baseURL)

	client := Client{
		baseURL:    *u,
		model:      model,
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}

	err := client.GenerateToken()
	if err != nil {
		return Client{}, err
	}
	return client, nil
}

func (c *Client) GetConcept(uuid string) ([]byte, error) {
	reqURL := c.baseURL
	q := "path=" + buildConceptPath(c.model, uuid) + "&properties=" + propertiesQueryParamValue
	reqURL.RawQuery = q

	resp, err := c.MakeRequest("GET", reqURL.String())
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}

	return body, nil
}

func (c *Client) MakeRequest(method, url string) (*http.Response, error) {
	if c.accessFailureCount >= maxAccessFailureCount {
		// We've failed to get a valid access token multiple times in a row, so just error out.
		return nil, errors.New("Failed to get a valid access token.")
	}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return resp, err
	}

	// We're checking if we got a 401, which would be because the token had expired.  If it has, generate a new
	// one and then make the request again.
	if resp.StatusCode == 401 {
		resp.Body.Close()
		c.accessFailureCount++
		c.GenerateToken()
		return c.MakeRequest(method, url)
	}
	c.accessFailureCount = 0
	return resp, err
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	UserName    string `json:"userName"`
	Issued      string `json:".issued"`
	Expires     string `json:".expires"`
}

// Tokens have a limited life, so to be safe we should generate a new one for each notification received.
func (c *Client) GenerateToken() error {
	data := url.Values{}
	data.Set("grant_type", "apikey")
	data.Set("key", c.apiKey)

	req, err := http.NewRequest("POST", slTokenURL, bytes.NewBufferString(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var tokenResponse TokenResponse
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&tokenResponse)
	if err != nil {
		return err
	}

	log.Debug("Setting Smartlogic access token")
	c.accessToken = tokenResponse.AccessToken
	return nil
}

func buildConceptPath(model, uuid string) string {
	/*
		Because the API call needs to be made as part of the 'path' query parameter, we need to escape the IRI twice,
		once to encode the IRI according to how Smartlogic needs it and once to encode it as a query parameter.
	*/
	thing := "<http://www.ft.com/thing/" + uuid + ">"
	encodedThing := url.QueryEscape(url.QueryEscape(thing))

	return "model:" + model + "/" + encodedThing
}
