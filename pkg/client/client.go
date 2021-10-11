package client

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/GustavoCaso/notion_workflows/pkg/types"
)

type NotionClient struct {
	httpClient http.Client
}

func NewHTTPClient(token string) NotionClient {
	httpClient := http.Client{
		Transport: &transport{
			underlyingTransport: http.DefaultTransport,
			token:               token,
		},
	}

	return NotionClient{
		httpClient: httpClient,
	}
}

func (c NotionClient) CreatePage(postBody io.Reader) types.PageResponse {
	request, err := http.NewRequest("POST", "https://api.notion.com/v1/pages", postBody)

	if err != nil {
		panic(err)
	}

	response, err := c.httpClient.Do(request)

	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	body, _ := ioutil.ReadAll(response.Body)

	var pageResponse types.PageResponse
	err = json.Unmarshal(body, &pageResponse)

	if err != nil {
		panic(err)
	}

	return pageResponse
}

type transport struct {
	token               string
	underlyingTransport http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", t.authToken())
	req.Header.Add("Notion-Version", "2021-08-16")
	req.Header.Add("Content-Type", "application/json")
	return t.underlyingTransport.RoundTrip(req)
}

func (t *transport) authToken() string {
	return fmt.Sprintf("Bearer %s", t.token)
}
