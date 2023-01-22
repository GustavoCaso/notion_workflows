package main

import (
	"bytes"
	"flag"
	"fmt"
	"time"

	"github.com/GustavoCaso/notion_workflows/pkg/client"
	"github.com/GustavoCaso/notion_workflows/pkg/types"
	"github.com/GustavoCaso/notion_workflows/pkg/utils"
)

const (
	dailyCheckDatabaseID    = "3b27a5d9-138b-4f50-9c7b-7a77224f0579"
	habitTrackerDatabaseID  = "9e031d67-5c5f-4183-9e1c-7e2e9330cae3"
	MonthTrackingDatabaseID = "83ab95f9-d1d9-489e-b761-8dfbe839ba37"
	WeekTrackingDatabaseID  = "8a9a5eb6-8d2c-49a5-a286-ececece9b2b5"
)

type trackingPageInfo struct {
	DatabaseID string
	Emoji      string
	Date       string
	Title      string
}

type dailyCheckPageInfo struct {
	DailyActivitiesPageID string
	Date                  string
	Title                 string
}

type weekPageInfo struct {
	StartDate            string
	EndDate              string
	Title                string
	HabitTrackingPageIDs []string
}

type monthPageInfo struct {
	Title       string
	StartDate   string
	EndDate     string
	WeekPageIDs []string
}

type trackingPagesIDs struct {
	dailyCheckPageIDs  []string
	habitTrakerPageIDs []string
}

type weekPageIDs []string

type week struct {
	days []time.Time
}

type monthData struct {
	startDate   string
	endDate     string
	name        string
	currentYear int
	weeks       map[int]*week
}

const DATE_FORMAT = "2006-01-02"

var month int
var year int
var filterQuery = string(`{
	"filter": {
			"property": "Name",
			"text": {
					"equals": "%s"
			}
	}
}`)

func init() {
	flag.IntVar(&month, "month", 0, "Month to create tracking pages")
	flag.IntVar(&year, "year", 0, "Year to create month pages")
}

func main() {
	flag.Parse()

	var currentTime time.Time
	now := time.Now()

	mothInt := int(month)
	YearInt := int(year)
	var currentYear int
	var currentMonth time.Month

	if YearInt == 0 {
		currentYear = now.Year()
	} else {
		currentYear = YearInt
	}

	if mothInt == 0 {
		currentMonth = now.Month()
	} else {
		currentMonth = time.Month(mothInt)
	}

	currentTime = time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, time.UTC)
	currentLocation := currentTime.Location()
	firstOfMonth := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, currentLocation)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	currentDay := firstOfMonth

	// Make sure the day of the first week starts on Monday even if is not on the same month
	currentDayOfTheWeek := currentDay.Weekday()
	if currentDayOfTheWeek != time.Monday {
		switch currentDayOfTheWeek {
		case time.Tuesday:
			currentDay = currentDay.AddDate(0, 0, -1)
		case time.Wednesday:
			currentDay = currentDay.AddDate(0, 0, -2)
		case time.Thursday:
			currentDay = currentDay.AddDate(0, 0, -3)
		case time.Friday:
			currentDay = currentDay.AddDate(0, 0, -4)
		case time.Saturday:
			currentDay = currentDay.AddDate(0, 0, -5)
		case time.Sunday:
			currentDay = currentDay.AddDate(0, 0, -6)
		}
	}

	month := monthData{
		startDate:   firstOfMonth.Format(DATE_FORMAT),
		endDate:     lastOfMonth.Format(DATE_FORMAT),
		name:        fmt.Sprintf("%s %d", firstOfMonth.Month().String(), currentYear),
		currentYear: currentYear,
		weeks:       map[int]*week{},
	}

	filled := false

	for !filled {
		fmt.Println(currentDay)
		_, weekNumber := currentDay.ISOWeek()
		fmt.Println(weekNumber)

		if month.weeks[weekNumber] == nil {
			month.weeks[weekNumber] = &week{}
		}

		month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, currentDay)

		if currentDay.Equal(lastOfMonth) {
			filled = true
		}

		currentDay = currentDay.AddDate(0, 0, 1)
	}

	// Make sure the last day of the last week is on Sunday even if is not on the same month
	_, weekNumber := lastOfMonth.ISOWeek()
	lastWeek := month.weeks[weekNumber]
	lastDayOfTheWeek := lastWeek.days[len(lastWeek.days)-1]
	lastDayOfMonth := lastDayOfTheWeek.Weekday()

	if lastDayOfMonth != time.Sunday {
		_, weekNumber := lastDayOfTheWeek.ISOWeek()

		switch lastDayOfMonth {
		case time.Monday:
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 1))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 2))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 3))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 4))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 5))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 6))
		case time.Tuesday:
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 1))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 2))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 3))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 4))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 5))
		case time.Wednesday:
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 1))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 2))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 3))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 4))
		case time.Thursday:
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 1))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 2))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 3))
		case time.Friday:
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 1))
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 2))
		case time.Saturday:
			month.weeks[weekNumber].days = append(month.weeks[weekNumber].days, lastDayOfTheWeek.AddDate(0, 0, 1))
		}
	}

	client := client.NewHTTPClient(utils.GetAuthenticationToken())
	generateMonthsPages(client, month)

	fmt.Println("Success")
}

func generateMonthsPages(client client.NotionClient, monthData monthData) {
	weekPageIDs := weekPageIDs{}

	for weekNumber, week := range monthData.weeks {
		pagesIds := trackingPagesIDs{}

		for _, day := range week.days {
			generateDayPages(client, day, &pagesIds)
		}

		fmt.Printf("Create week page for SartDate: %s, EndDate: %s\n", week.days[0].Format(DATE_FORMAT), week.days[len(week.days)-1].Format(DATE_FORMAT))
		weekPageInfo := weekPageInfo{
			StartDate:            week.days[0].Format(DATE_FORMAT),
			EndDate:              week.days[len(week.days)-1].Format(DATE_FORMAT),
			Title:                fmt.Sprintf("Week %d (%d)", weekNumber, monthData.currentYear),
			HabitTrackingPageIDs: pagesIds.habitTrakerPageIDs,
		}

		weekPageId := createWeekPage(client, weekPageInfo)
		weekPageIDs = append(weekPageIDs, weekPageId)
	}

	monthPageInfo := monthPageInfo{
		Title:       monthData.name,
		StartDate:   monthData.startDate,
		EndDate:     monthData.endDate,
		WeekPageIDs: weekPageIDs,
	}

	createMonthPage(client, monthPageInfo)
}

func generateDayPages(client client.NotionClient, currentDay time.Time, pageIds *trackingPagesIDs) {
	date := currentDay.Format(DATE_FORMAT)
	title := fmt.Sprintf("%02d/%02d/%d", currentDay.Day(), currentDay.Month(), currentDay.Year())

	habitPageInfo := trackingPageInfo{
		DatabaseID: habitTrackerDatabaseID,
		Emoji:      "👟",
		Date:       date,
		Title:      title,
	}

	habitTrackerPageID := createTrackingPage(client, habitPageInfo)

	daylyCheckPageInfo := dailyCheckPageInfo{
		DailyActivitiesPageID: habitTrackerPageID,
		Date:                  date,
		Title:                 title,
	}

	dailyCheckPageID := createDailyCheckPage(client, daylyCheckPageInfo)
	fmt.Printf("Success creating daily check and tracking for %s\n", currentDay)
	pageIds.dailyCheckPageIDs = append(pageIds.dailyCheckPageIDs, dailyCheckPageID)
	pageIds.habitTrakerPageIDs = append(pageIds.habitTrakerPageIDs, habitTrackerPageID)
}

func createWeekPage(client client.NotionClient, pageInfo weekPageInfo) string {
	filter := fmt.Sprintf(filterQuery, pageInfo.Title)
	findResponse := client.FindPages(WeekTrackingDatabaseID, bytes.NewBuffer([]byte(filter)))
	pageFound := len(findResponse) == 1

	if pageFound {
		habitTrackerRelation := findResponse[0].Properties["Habit Tracker (Relation)"]["relation"].([]interface{})
		for _, IDmap := range habitTrackerRelation {
			IDmapCasted := IDmap.(map[string]interface{})
			id := IDmapCasted["id"].(string)
			if !utils.Contains(id, pageInfo.HabitTrackingPageIDs) {
				pageInfo.HabitTrackingPageIDs = append(pageInfo.HabitTrackingPageIDs, id)
			}
		}
	}

	buf, err := utils.ExecuteTemplate("templates/week_page.json.txt", "createWeekPage", pageInfo)
	if err != nil {
		panic(err)
	}

	var response types.PageResponse
	if pageFound {
		response = client.UpdatePage(findResponse[0].ID, bytes.NewBuffer(buf.Bytes()))
		fmt.Printf("Success updating week page for %+v\n", pageInfo)
	} else {
		response = client.CreatePage(bytes.NewBuffer(buf.Bytes()))
		fmt.Printf("Success creating week page for %+v\n", pageInfo)
	}

	return response.ID
}

func createMonthPage(client client.NotionClient, pageInfo monthPageInfo) string {
	filter := fmt.Sprintf(filterQuery, pageInfo.Title)
	findResponse := client.FindPages(MonthTrackingDatabaseID, bytes.NewBuffer([]byte(filter)))
	pageFound := len(findResponse) == 1

	if pageFound {
		weekPagesRelation := findResponse[0].Properties["Weeks"]["relation"].([]interface{})
		for _, IDmap := range weekPagesRelation {
			IDmapCasted := IDmap.(map[string]interface{})
			id := IDmapCasted["id"].(string)
			if !utils.Contains(id, pageInfo.WeekPageIDs) {
				pageInfo.WeekPageIDs = append(pageInfo.WeekPageIDs, id)
			}
		}
	}

	buf, err := utils.ExecuteTemplate("templates/month_page.json.txt", "createMonthPage", pageInfo)
	if err != nil {
		panic(err)
	}

	var response types.PageResponse
	if pageFound {
		response = client.UpdatePage(findResponse[0].ID, bytes.NewBuffer(buf.Bytes()))

		fmt.Printf("Success updating month page for %+v\n", pageInfo)

	} else {
		response = client.CreatePage(bytes.NewBuffer(buf.Bytes()))

		fmt.Printf("Success creating month page for %+v\n", pageInfo)
	}

	return response.ID
}

func createTrackingPage(client client.NotionClient, pageInfo trackingPageInfo) string {
	buf, err := utils.ExecuteTemplate("templates/tracking_page.json", "createTrackingPage", pageInfo)
	if err != nil {
		panic(err)
	}

	filter := fmt.Sprintf(filterQuery, pageInfo.Title)
	response := client.FindOrCreatePage(pageInfo.DatabaseID, bytes.NewBuffer([]byte(filter)), bytes.NewBuffer(buf.Bytes()))

	return response.ID
}

func createDailyCheckPage(client client.NotionClient, pageInfo dailyCheckPageInfo) string {
	buf, err := utils.ExecuteTemplate("templates/daily_check_page.json", "createDailyCheckPage", pageInfo)
	if err != nil {
		panic(err)
	}

	filter := fmt.Sprintf(filterQuery, pageInfo.Title)
	response := client.FindOrCreatePage(dailyCheckDatabaseID, bytes.NewBuffer([]byte(filter)), bytes.NewBuffer(buf.Bytes()))

	return response.ID
}
