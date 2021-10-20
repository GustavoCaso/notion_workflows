package main

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/GustavoCaso/notion_workflows/pkg/client"
	"github.com/GustavoCaso/notion_workflows/pkg/utils"
)

const (
	habitTrackerId   = "9e031d67-5c5f-4183-9e1c-7e2e9330cae3"
	alcoholTrackerId = "ec908068-203c-4882-acaa-2b3032a861c7"
)

type trackingPageInfo struct {
	DatabaseID string
	Emoji      string
	Date       string
	Title      string
}

type dailyCheckPageInfo struct {
	AlcoholTrackerPageID  string
	DailyActivitiesPageID string
	Date                  string
	Title                 string
}

func main() {
	client := client.NewHTTPClient(utils.GetAuthenticationToken())
	currentTime := time.Now()

	date := currentTime.Format("2006-01-02")
	title := fmt.Sprintf("%02d/%02d/%d", currentTime.Day(), currentTime.Month(), currentTime.Year())

	alcoholPageInfo := trackingPageInfo{
		DatabaseID: alcoholTrackerId,
		Emoji:      "🍺",
		Date:       date,
		Title:      title,
	}

	habitPageInfo := trackingPageInfo{
		DatabaseID: habitTrackerId,
		Emoji:      "👟",
		Date:       date,
		Title:      title,
	}

	alcoholTrackerPageID := createTrackingPage(client, alcoholPageInfo)
	habitTrackerPageID := createTrackingPage(client, habitPageInfo)

	daylyCheckPageInfo := dailyCheckPageInfo{
		AlcoholTrackerPageID:  alcoholTrackerPageID,
		DailyActivitiesPageID: habitTrackerPageID,
		Date:                  date,
		Title:                 title,
	}

	createDailyCheckPage(client, daylyCheckPageInfo)
	fmt.Println("Success")
}

func createTrackingPage(client client.NotionClient, pageInfo trackingPageInfo) string {
	data, err := os.ReadFile("templates/tracking_page.json")
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	t := template.Must(template.New("createTrackingPage").Parse(string(data)))
	t.Execute(&buf, pageInfo)

	filter := fmt.Sprintf(string(`{
    "filter": {
        "property": "Name",
        "text": {
            "equals": "%s"
        }
    }
}`), pageInfo.Title)
	response := client.FindOrCreatePage(pageInfo.DatabaseID, bytes.NewBuffer([]byte(filter)), bytes.NewBuffer(buf.Bytes()))

	return response.ID
}

func createDailyCheckPage(client client.NotionClient, pageInfo dailyCheckPageInfo) string {
	data, err := os.ReadFile("templates/daily_check_page.json")
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	t := template.Must(template.New("createDailyCheckPage").Parse(string(data)))
	t.Execute(&buf, pageInfo)

	filter := fmt.Sprintf(string(`{
    "filter": {
        "property": "Name",
        "text": {
            "equals": "%s"
        }
    }
}`), pageInfo.Title)

	response := client.FindOrCreatePage("3b27a5d9-138b-4f50-9c7b-7a77224f0579", bytes.NewBuffer([]byte(filter)), bytes.NewBuffer(buf.Bytes()))

	return response.ID
}
