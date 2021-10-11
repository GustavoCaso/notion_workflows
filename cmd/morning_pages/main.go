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
	notionAPIURL         = "https://api.notion.com/v1"
	dailyCheckDatabaseID = "3b27a5d9-138b-4f50-9c7b-7a77224f0579"
	habitTrackerId       = "9e031d67-5c5f-4183-9e1c-7e2e9330cae3"
	alcoholTrackerId     = "ec908068-203c-4882-acaa-2b3032a861c7"
)

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
	client := client.NewHTTPClient(utils.GetAuthenticationToken())
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

func createEmptyPage(client client.NotionClient, pageInfo childPageInfo) string {
	data, err := os.ReadFile("templates/create_page.json")
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	t := template.Must(template.New("createPage").Parse(string(data)))
	t.Execute(&buf, pageInfo)

	response := client.CreatePage(bytes.NewBuffer(buf.Bytes()))

	return response.ID
}

func createPageWithContent(client client.NotionClient, pageInfo parentPageInfo) string {
	data, err := os.ReadFile("templates/create_page_with_content.json")
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	t := template.Must(template.New("createPageWithContent").Parse(string(data)))
	t.Execute(&buf, pageInfo)

	response := client.CreatePage(bytes.NewBuffer(buf.Bytes()))

	return response.ID
}
