package client

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/GustavoCaso/notion_workflows/pkg/types"
)

const notionAPIURL = "https://api.notion.com/v1"

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

func (c NotionClient) FindOrCreatePage(databaseId string, pageQuery, pageBody io.Reader) types.PageResponse {
	fmt.Printf("Querying DB %s with Query %s\n", databaseId, pageQuery)
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/databases/%s/query", notionAPIURL, databaseId), pageQuery)

	if err != nil {
		panic(err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		body, _ := ioutil.ReadAll(response.Body)

		requestBodyReader, _ := request.GetBody()

		requestBody, _ := ioutil.ReadAll(requestBodyReader)
		panic(fmt.Errorf("Failed to list pages. Error %s. QueryBody sent %s", body, requestBody))
	}

	body, _ := ioutil.ReadAll(response.Body)

	var listPageResponse types.ListPageResponse
	err = json.Unmarshal(body, &listPageResponse)

	if err != nil {
		panic(err)
	}

	fmt.Printf("Querying DB Resulst%s\n", listPageResponse)

	resultsLength := len(listPageResponse.Results)

	if resultsLength == 1 {
		return listPageResponse.Results[0]
	}

	if resultsLength > 1 {
		panic(fmt.Errorf("multiple pages returns from querying the database: %s", databaseId))
	}

	return c.CreatePage(pageBody)
}

func (c NotionClient) FindPages(databaseId string, pageQuery io.Reader) []types.PageResponse {
	fmt.Printf("Querying DB %s with Query %s\n", databaseId, pageQuery)
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/databases/%s/query", notionAPIURL, databaseId), pageQuery)

	if err != nil {
		panic(err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		body, _ := ioutil.ReadAll(response.Body)

		requestBodyReader, _ := request.GetBody()

		requestBody, _ := ioutil.ReadAll(requestBodyReader)
		panic(fmt.Errorf("Failed to list pages. Error %s. QueryBody sent %s", body, requestBody))
	}

	body, _ := ioutil.ReadAll(response.Body)

	var listPageResponse types.ListPageResponse
	err = json.Unmarshal(body, &listPageResponse)

	if err != nil {
		panic(err)
	}

	if len(listPageResponse.Results) > 0 {
		return listPageResponse.Results
	}
	return []types.PageResponse{}
}

func (c NotionClient) UpdatePage(pageID string, pageBody io.Reader) types.PageResponse {
	fmt.Printf("Updating Page %s with Body %s\n", pageID, pageBody)
	request, err := http.NewRequest("PATCH", fmt.Sprintf("%s/pages/%s", notionAPIURL, pageID), pageBody)

	if err != nil {
		panic(err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		body, _ := ioutil.ReadAll(response.Body)

		requestBodyReader, _ := request.GetBody()

		requestBody, _ := ioutil.ReadAll(requestBodyReader)
		panic(fmt.Errorf("Failed to update page. Error %s. QueryBody sent %s", body, requestBody))
	}

	body, _ := ioutil.ReadAll(response.Body)

	var pageResponse types.PageResponse
	err = json.Unmarshal(body, &pageResponse)

	if err != nil {
		panic(err)
	}

	return pageResponse
}

func (c NotionClient) CreatePage(postBody io.Reader) types.PageResponse {
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/pages", notionAPIURL), postBody)

	if err != nil {
		panic(err)
	}

	response, err := c.httpClient.Do(request)

	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	if response.StatusCode != 200 {
		body, _ := ioutil.ReadAll(response.Body)

		requestBodyReader, _ := request.GetBody()

		requestBody, _ := ioutil.ReadAll(requestBodyReader)
		panic(fmt.Errorf("Failed to create page. Error %s. PostBody sent %s", body, requestBody))
	}

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
