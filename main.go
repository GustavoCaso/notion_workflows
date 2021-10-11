package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

const (
	notionAPIURL         = "https://api.notion.com/v1"
	dailyCheckDatabaseID = "3b27a5d9-138b-4f50-9c7b-7a77224f0579"
	habitTrackerId       = "9e031d67-5c5f-4183-9e1c-7e2e9330cae3"
	alcoholTrackerId     = "ec908068-203c-4882-acaa-2b3032a861c7"
)

type pageResponse struct {
	ID string `json:"id"`
}

type childPageInfo struct {
	DatabaseID string
	Emoji      string
	Date       string
	Title      string
}

type parentPageInfo struct {
	AlcoholTrackerPageID  string
	DailyActivitiesPageID string
	Date                  string
	Title                 string
}

func main() {
	client := newHTTPClient(getAuthenticationToken())
	currentTime := time.Now()

	date := currentTime.Format("2006-01-02")
	title := fmt.Sprintf("%02d/%02d/%d", currentTime.Day(), currentTime.Month(), currentTime.Year())

	alcoholPageInfo := childPageInfo{
		DatabaseID: alcoholTrackerId,
		Emoji:      "🍺",
		Date:       date,
		Title:      title,
	}

	habitPageInfo := childPageInfo{
		DatabaseID: habitTrackerId,
		Emoji:      "👟",
		Date:       date,
		Title:      title,
	}

	alcoholTrackerPageID := createEmptyPage(client, alcoholPageInfo)
	habitTrackerPageID := createEmptyPage(client, habitPageInfo)

	daylyCheckPageInfo := parentPageInfo{
		AlcoholTrackerPageID:  alcoholTrackerPageID,
		DailyActivitiesPageID: habitTrackerPageID,
		Date:                  date,
		Title:                 title,
	}

	createPageWithContent(client, daylyCheckPageInfo)
	fmt.Println("Success")
}

func createEmptyPage(client http.Client, pageInfo childPageInfo) string {
	data, err := os.ReadFile("create_page.json")
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	t := template.Must(template.New("createPage").Parse(string(data)))
	t.Execute(&buf, pageInfo)

	request, err := http.NewRequest("POST", "https://api.notion.com/v1/pages", bytes.NewBuffer(buf.Bytes()))

	if err != nil {
		panic(err)
	}

	response, err := client.Do(request)

	if err != nil {
		panic(err)
	}

	defer response.Body.Close()

	body, _ := ioutil.ReadAll(response.Body)

	var pageResponse pageResponse
	err = json.Unmarshal(body, &pageResponse)

	if err != nil {
		panic(err)
	}

	return pageResponse.ID
}

func createPageWithContent(client http.Client, pageInfo parentPageInfo) string {
	data, err := os.ReadFile("create_page_with_content.json")
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	t := template.Must(template.New("createPageWithContent").Parse(string(data)))
	t.Execute(&buf, pageInfo)

	request, err := http.NewRequest("POST", "https://api.notion.com/v1/pages", bytes.NewBuffer(buf.Bytes()))

	if err != nil {
		panic(err)
	}

	// fmt.Println(formatRequest(request))

	response, err := client.Do(request)

	if err != nil {
		panic(err)
	}

	defer response.Body.Close()

	body, _ := ioutil.ReadAll(response.Body)

	var pageResponse pageResponse
	err = json.Unmarshal(body, &pageResponse)

	if err != nil {
		panic(err)
	}

	return pageResponse.ID
}

func getAuthenticationToken() string {
	value := os.Getenv("MORNING_WORKFLOW_API_TOKEN")
	if value == "" {
		panic(errors.New("The ENV variable MORNING_WORKFLOW_API_TOKEN must be set"))
	}
	return value
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

func newHTTPClient(token string) http.Client {
	return http.Client{
		Transport: &transport{
			underlyingTransport: http.DefaultTransport,
			token:               token,
		},
	}
}
