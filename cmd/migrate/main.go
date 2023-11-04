package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dstotijn/go-notion"
	"github.com/itchyny/timefmt-go"
)

type cache struct {
	storage map[string]string
	mu      sync.RWMutex
}

func (c *cache) Get(value string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.storage[value]
	return val, ok
}

func (c *cache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.storage[key] = value
}

func newCache() *cache {
	storage := map[string]string{}
	return &cache{
		storage: storage,
	}
}

var mentionCache = newCache()

type job struct {
	run func() error
}

type errJob struct {
	job job
	err error
}

type queue struct {
	jobs   chan job
	cancel context.CancelFunc
	ctx    context.Context
}

func newQueue() *queue {
	ctx, cancel := context.WithCancel(context.Background())

	return &queue{
		jobs:   make(chan job),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (q *queue) addJobs(jobs []job) {
	var wg sync.WaitGroup
	wg.Add(len(jobs))

	for _, pageJob := range jobs {
		go func(job job) {
			q.addJob(job)
			wg.Done()
		}(pageJob)
	}

	go func() {
		wg.Wait()
		q.cancel()
	}()
}

func (q *queue) addJob(job job) {
	q.jobs <- job
}

type worker struct {
	queue     *queue
	errorJobs []errJob
}

func (w *worker) doWork() bool {
	for {
		select {
		case <-w.queue.ctx.Done():
			fmt.Print("Finish migrating pages\n")
			return true
		case job := <-w.queue.jobs:
			err := job.run()
			if err != nil {
				errJob := errJob{
					job: job,
					err: err,
				}
				w.errorJobs = append(w.errorJobs, errJob)
				continue
			}
		}
	}
}

type pathAttributes map[string]string

var databaseIDUsage = `notion database ID to migrate.
If you want to specify the propeties to convert to frontmater use a colon and provide a comma separated list. Ex ID:name,date
If you rather want to provide a skip list separate the ID and the skip list using >. Ex ID>day of the week,date
`

var token = flag.String("token", os.Getenv("NOTION_TOKEN"), "notion token")
var databaseID = flag.String("id", os.Getenv("NOTION_DATABASE_ID"), databaseIDUsage)
var obsidianVault = flag.String("vault", os.Getenv("OBSIDIAN_VAULT_PATH"), "Obsidian vault location")
var pagePath = flag.String("path", "", "Page path in which to store the pages. Support selecting different page attribute and formatting")

func main() {
	flag.Parse()

	if empty(token) {
		flag.Usage()
		fmt.Println("You must provide the notion token to run the script")
		os.Exit(1)
	}

	if empty(databaseID) {
		flag.Usage()
		fmt.Println("You must provide the notion database id to run the script")
		os.Exit(1)
	}

	databaseIDCopy := *databaseID
	dbPropertiesSet := map[string]bool{}
	dbPropertiesSkipSet := map[string]bool{}

	results := strings.Split(databaseIDCopy, ":")
	if len(results) > 1 {
		dbProperties := strings.Split(results[1], ",")
		for _, dbProp := range dbProperties {
			dbPropertiesSet[strings.ToLower(dbProp)] = true
		}
	}

	results = strings.Split(databaseIDCopy, ">")
	if len(results) > 1 {
		dbPropertiesToSkip := strings.Split(results[1], ",")
		for _, dbProp := range dbPropertiesToSkip {
			dbPropertiesSkipSet[strings.ToLower(dbProp)] = true
		}
	}

	databaseID = &results[0]

	if len(dbPropertiesSet) > 0 && len(dbPropertiesSkipSet) > 0 {
		fmt.Println("You can not provide both skip list and include list for DB properties")
		os.Exit(1)
	}

	if empty(obsidianVault) {
		flag.Usage()
		fmt.Println("You must provide the obisidian vault path to run the script")
		os.Exit(1)
	}

	pagePathFilters := pathAttributes{}
	if !empty(pagePath) {
		pagePathResults := strings.Split(*pagePath, ",")
		for _, pagePathAttribute := range pagePathResults {
			pageWithFormatOptions := strings.Split(pagePathAttribute, ":")
			if len(pageWithFormatOptions) > 1 {
				pagePathFilters[strings.ToLower(pageWithFormatOptions[0])] = pageWithFormatOptions[1]
			} else {
				pagePathFilters[strings.ToLower(pageWithFormatOptions[0])] = ""
			}
		}
	}

	client := notion.NewClient(*token)

	filterTime, _ := time.Parse(time.RFC3339, "2020-01-01")
	query := &notion.DatabaseQuery{
		Filter: &notion.DatabaseQueryFilter{
			Property: "Date",
			DatabaseQueryPropertyFilter: notion.DatabaseQueryPropertyFilter{
				Date: &notion.DatePropertyFilter{
					After: &filterTime,
				},
			},
		},
		Sorts: []notion.DatabaseQuerySort{
			{
				Property:  "Date",
				Direction: notion.SortDirAsc,
			},
		},
	}

	pages, _ := fetchNotionDBPages(client, query)

	// Print progress bar

	var jobs []job

	queue := newQueue()

	for _, page := range pages {
		// We need to do this, because variables declared in for loops are passed by reference.
		// Otherwise, our closure will always receive the last item from the page.
		newPage := page

		job := job{
			run: func() error {
				path := filePath(newPage, pagePathFilters)
				return fetchAndSaveToObsidianVault(client, newPage, dbPropertiesSet, dbPropertiesSkipSet, path)
			},
		}

		jobs = append(jobs, job)
	}

	// enequeue page to download and parse
	queue.addJobs(jobs)
	// make sure that we take into a ccount the rate limit constraint from notion

	worker := worker{
		queue: queue,
	}

	worker.doWork()

	for _, errJob := range worker.errorJobs {
		fmt.Printf("an error ocurred when processing a page %v\n", errors.Unwrap(errJob.err))
	}
}

func empty(v *string) bool {
	return *v == ""
}

func filePath(page notion.Page, pagePathProperties pathAttributes) string {
	properties := page.Properties.(notion.DatabasePageProperties)
	var str string
	for key, value := range properties {
		val, ok := pagePathProperties[strings.ToLower(key)]
		if ok {
			switch value.Type {
			case notion.DBPropTypeDate:
				date := value.Date.Start
				if val != "" {
					str += timefmt.Format(date.Time, "%Y/%B/%d-%A")
				}
			default:
				panic("not suported")
			}
		}
	}
	fileName := fmt.Sprintf("%s.md", str)
	return path.Join(*obsidianVault, fileName)
}

func fetchNotionDBPages(client *notion.Client, query *notion.DatabaseQuery) ([]notion.Page, error) {
	notionResponse, err := client.QueryDatabase(context.Background(), *databaseID, query)
	if err != nil {
		panic(err)
	}

	result := []notion.Page{}

	result = append(result, notionResponse.Results...)

	for notionResponse.HasMore {
		query.StartCursor = *notionResponse.NextCursor

		notionResponse, err = client.QueryDatabase(context.Background(), *databaseID, query)
		if err != nil {
			panic(err)
		}

		result = append(result, notionResponse.Results...)
	}

	return result, nil
}

func fetchAndSaveToObsidianVault(client *notion.Client, page notion.Page, pagePropertiesToInclude, pagePropertiesToSkip map[string]bool, obsidianPath string) error {
	pageBlocks, err := client.FindBlockChildrenByID(context.Background(), page.ID, nil)
	if err != nil {
		return fmt.Errorf("failed to extract children blocks for block ID %s. error: %w", page.ID, err)
	}

	if err := os.MkdirAll(filepath.Dir(obsidianPath), 0770); err != nil {
		return fmt.Errorf("failed to create the necessary directories in for the Obsidian vault.  error: %w", err)
	}

	f, err := os.Create(obsidianPath)
	if err != nil {
		return fmt.Errorf("failed to create the markdown file %s. error: %w", path.Base(obsidianPath), err)
	}

	defer f.Close()

	// create new buffer
	buffer := bufio.NewWriter(f)

	props := page.Properties.(notion.DatabasePageProperties)

	selectedProps := make(notion.DatabasePageProperties)

	if len(pagePropertiesToInclude) > 0 {
		for propName, propValue := range props {
			if pagePropertiesToInclude[strings.ToLower(propName)] {
				selectedProps[propName] = propValue
			}
		}
	}

	if len(pagePropertiesToSkip) > 0 {
		for propName, propValue := range props {
			if !pagePropertiesToSkip[strings.ToLower(propName)] {
				selectedProps[propName] = propValue
			}
		}
	}

	propertiesToFrontMatter(selectedProps, buffer)

	err = pageToMarkdown(client, pageBlocks.Results, buffer, false)

	if err != nil {
		return fmt.Errorf("failed to convert page tp markdown. error: %w", err)
	}

	if err = buffer.Flush(); err != nil {
		return fmt.Errorf("failed to write into the markdown file %s. error: %w", path.Base(obsidianPath), err)
	}

	return nil
}

func pageToMarkdown(client *notion.Client, blocks []notion.Block, buffer *bufio.Writer, indent bool) error {
	var err error

	for _, object := range blocks {
		switch block := object.(type) {
		case *notion.Heading1Block:
			if indent {
				buffer.WriteString("	# ")
			} else {
				buffer.WriteString("# ")
			}
			if err = writeRichText(client, buffer, block.RichText); err != nil {
				return err
			}
			buffer.WriteString("\n")
			if err = writeChrildren(client, object, buffer); err != nil {
				return err
			}
		case *notion.Heading2Block:
			if indent {
				buffer.WriteString("	## ")
			} else {
				buffer.WriteString("## ")
			}
			if err = writeRichText(client, buffer, block.RichText); err != nil {
				return err
			}
			buffer.WriteString("\n")
			if err = writeChrildren(client, object, buffer); err != nil {
				return err
			}
		case *notion.Heading3Block:
			if indent {
				buffer.WriteString("	### ")
			} else {
				buffer.WriteString("### ")
			}
			if err = writeRichText(client, buffer, block.RichText); err != nil {
				return err
			}
			buffer.WriteString("\n")
			if err = writeChrildren(client, object, buffer); err != nil {
				return err
			}
		case *notion.ToDoBlock:
			if indent {
				if *block.Checked {
					buffer.WriteString("	- [x] ")
				} else {
					buffer.WriteString("	- [ ] ")
				}
			} else {
				if *block.Checked {
					buffer.WriteString("- [x] ")
				} else {
					buffer.WriteString("- [ ] ")
				}
			}
			if err = writeRichText(client, buffer, block.RichText); err != nil {
				return err
			}
			buffer.WriteString("\n")
			if err = writeChrildren(client, object, buffer); err != nil {
				return err
			}
		case *notion.ParagraphBlock:
			if len(block.RichText) > 0 {
				if indent {
					buffer.WriteString("	")
					if err = writeRichText(client, buffer, block.RichText); err != nil {
						return err
					}
				} else {
					if err = writeRichText(client, buffer, block.RichText); err != nil {
						return err
					}
				}
			}
			buffer.WriteString("\n")
			if err = writeChrildren(client, object, buffer); err != nil {
				return err
			}
		case *notion.BulletedListItemBlock:
			if indent {
				buffer.WriteString("	- ")
			} else {
				buffer.WriteString("- ")
			}
			if err = writeRichText(client, buffer, block.RichText); err != nil {
				return err
			}
			buffer.WriteString("\n")
			if err = writeChrildren(client, object, buffer); err != nil {
				return err
			}
		case *notion.NumberedListItemBlock:
			if indent {
				buffer.WriteString("	- ")
			} else {
				buffer.WriteString("- ")
			}
			if err = writeRichText(client, buffer, block.RichText); err != nil {
				return err
			}
			buffer.WriteString("\n")
			if err = writeChrildren(client, object, buffer); err != nil {
				return err
			}
		case *notion.CalloutBlock:
			if indent {
				buffer.WriteString("	> [!")
			} else {
				buffer.WriteString("> [!")
			}
			if len(*block.Icon.Emoji) > 0 {
				buffer.WriteString(*block.Icon.Emoji)
			}
			if err = writeRichText(client, buffer, block.RichText); err != nil {
				return err
			}
			buffer.WriteString("]")
			buffer.WriteString("\n")
		case *notion.ToggleBlock:
			if indent {
				buffer.WriteString("	- ")
			} else {
				buffer.WriteString("- ")
			}
			if err = writeRichText(client, buffer, block.RichText); err != nil {
				return err
			}
			buffer.WriteString("\n")
			if err = writeChrildren(client, object, buffer); err != nil {
				return err
			}
		case *notion.QuoteBlock:
			if indent {
				buffer.WriteString("	> ")
			} else {
				buffer.WriteString("> ")
			}
			if err = writeRichText(client, buffer, block.RichText); err != nil {
				return err
			}
			buffer.WriteString("\n")
			if err = writeChrildren(client, object, buffer); err != nil {
				return err
			}
		case *notion.FileBlock:
			if block.Type == notion.FileTypeExternal {
				if indent {
					buffer.WriteString(fmt.Sprintf("	![](%s)", block.External.URL))
				} else {
					buffer.WriteString(fmt.Sprintf("![](%s)", block.External.URL))
				}
			}
			buffer.WriteString("\n")
		case *notion.DividerBlock:
			buffer.WriteString("\n")
		case *notion.ChildPageBlock:
		case *notion.CodeBlock:
			buffer.WriteString("```")
			buffer.WriteString(*block.Language)
			buffer.WriteString("\n")
			if err = writeRichText(client, buffer, block.RichText); err != nil {
				return err
			}
			buffer.WriteString("\n")
			buffer.WriteString("```")
			buffer.WriteString("\n")
		case *notion.ImageBlock:
			if block.Type == notion.FileTypeExternal {
				if indent {
					buffer.WriteString(fmt.Sprintf("	![](%s)", block.External.URL))
				} else {
					buffer.WriteString(fmt.Sprintf("![](%s)", block.External.URL))
				}
			}
			buffer.WriteString("\n")
		case *notion.VideoBlock:
			if block.Type == notion.FileTypeExternal {
				if indent {
					buffer.WriteString(fmt.Sprintf("	![](%s)", block.External.URL))
				} else {
					buffer.WriteString(fmt.Sprintf("![](%s)", block.External.URL))
				}
			}
			buffer.WriteString("\n")
		case *notion.EmbedBlock:
			if indent {
				buffer.WriteString(fmt.Sprintf("	![](%s)", block.URL))
			} else {
				buffer.WriteString(fmt.Sprintf("![](%s)", block.URL))
			}
			buffer.WriteString("\n")
		case *notion.BookmarkBlock:
			if indent {
				buffer.WriteString(fmt.Sprintf("	![](%s)", block.URL))
			} else {
				buffer.WriteString(fmt.Sprintf("![](%s)", block.URL))
			}
			buffer.WriteString("\n")
		case *notion.ChildDatabaseBlock:
			if indent {
				buffer.WriteString(fmt.Sprintf("	%s", block.Title))
			} else {
				buffer.WriteString(block.Title)
			}
			buffer.WriteString("\n")
		default:
			return fmt.Errorf("block not supported: %+v", block)
		}
	}

	return nil
}

func propertiesToFrontMatter(propertites notion.DatabasePageProperties, buffer *bufio.Writer) {
	buffer.WriteString("---\n")
	for key, value := range propertites {
		switch value.Type {
		case notion.DBPropTypeTitle:
			buffer.WriteString(fmt.Sprintf("%s: %s\n", key, extractPlainTextFromRichText(value.Title)))
		case notion.DBPropTypeRichText:
			buffer.WriteString(fmt.Sprintf("%s: %s\n", key, extractPlainTextFromRichText(value.RichText)))
		case notion.DBPropTypeNumber:
			buffer.WriteString(fmt.Sprintf("%s: %f\n", key, *value.Number))
		case notion.DBPropTypeSelect:
			if value.Select != nil {
				buffer.WriteString(fmt.Sprintf("%s: %s\n", key, value.Select.Name))
			}
		case notion.DBPropTypeMultiSelect:
			options := []string{}
			for _, option := range value.MultiSelect {
				options = append(options, option.Name)
			}

			buffer.WriteString(fmt.Sprintf("%s: [%s]\n", key, strings.Join(options[:], ",")))
		case notion.DBPropTypeDate:
			if value.Date != nil {
				if value.Date.Start.HasTime() {
					buffer.WriteString(fmt.Sprintf("%s: %s\n", key, value.Date.Start.Format("2006-01-02T15:04:05")))
				} else {
					buffer.WriteString(fmt.Sprintf("%s: %s\n", key, value.Date.Start.Format("2006-01-02")))
				}
			}
		case notion.DBPropTypePeople:
		case notion.DBPropTypeFiles:
		case notion.DBPropTypeCheckbox:
			buffer.WriteString(fmt.Sprintf("%s: %t\n", key, *value.Checkbox))
		case notion.DBPropTypeURL:
			buffer.WriteString(fmt.Sprintf("%s: %s\n", key, *value.URL))
		case notion.DBPropTypeEmail:
			buffer.WriteString(fmt.Sprintf("%s: %s\n", key, *value.Email))
		case notion.DBPropTypePhoneNumber:
			buffer.WriteString(fmt.Sprintf("%s: %s\n", key, *value.PhoneNumber))
		case notion.DBPropTypeStatus:
			buffer.WriteString(fmt.Sprintf("%s: %s\n", key, value.Status.Name))
		case notion.DBPropTypeFormula:
		case notion.DBPropTypeRelation:
		case notion.DBPropTypeRollup:
			switch value.Rollup.Type {
			case notion.RollupResultTypeNumber:
				buffer.WriteString(fmt.Sprintf("%s: %f\n", key, *value.Rollup.Number))
			case notion.RollupResultTypeDate:
				if value.Rollup.Date.Start.HasTime() {
					buffer.WriteString(fmt.Sprintf("%s: %s\n", key, value.Rollup.Date.Start.Format("2006-01-02T15:04:05")))
				} else {
					buffer.WriteString(fmt.Sprintf("%s: %s\n", key, value.Rollup.Date.Start.Format("2006-01-02")))
				}
			}
		case notion.DBPropTypeCreatedTime:
			buffer.WriteString(fmt.Sprintf("%s: %s\n", key, value.CreatedTime.String()))
		case notion.DBPropTypeCreatedBy:
			buffer.WriteString(fmt.Sprintf("%s: %s\n", key, value.CreatedBy.Name))
		case notion.DBPropTypeLastEditedTime:
			buffer.WriteString(fmt.Sprintf("%s: %s\n", key, value.LastEditedTime.String()))
		case notion.DBPropTypeLastEditedBy:
			buffer.WriteString(fmt.Sprintf("%s: %s\n", key, value.LastEditedBy.Name))
		default:
		}
	}
	buffer.WriteString("---\n")
}

func writeChrildren(client *notion.Client, block notion.Block, buffer *bufio.Writer) error {
	if block.HasChildren() {
		pageBlocks, err := client.FindBlockChildrenByID(context.Background(), block.ID(), nil)
		if err != nil {
			return fmt.Errorf("failed to extract children blocks for block ID %s. error: %w", block.ID(), err)
		}
		return pageToMarkdown(client, pageBlocks.Results, buffer, true)
	}

	return nil
}

func writeRichText(client *notion.Client, buffer *bufio.Writer, richText []notion.RichText) error {
	for _, text := range richText {
		switch text.Type {
		case notion.RichTextTypeText:
			if text.Annotations.Color == notion.ColorDefault {
				buffer.WriteString(text.Text.Content)
			} else {
				buffer.WriteString("==")
				buffer.WriteString(text.Text.Content)
				buffer.WriteString("==")
			}
		case notion.RichTextTypeMention:
			switch text.Mention.Type {
			case notion.MentionTypePage:
				val, ok := mentionCache.Get(text.Mention.Page.ID)
				if ok {
					buffer.WriteString("[[")
					buffer.WriteString(val)
					buffer.WriteString("]]")
				} else {
					pageTitle := text.PlainText

					mentionPage, err := client.FindPageByID(context.Background(), text.Mention.Page.ID)
					if err != nil {
						return fmt.Errorf("failed to find mention page %s.  error: %w", text.Mention.Page.ID, err)
					}

					if mentionPage.Parent.Type == notion.ParentTypeDatabase {
						dbPage, err := client.FindDatabaseByID(context.Background(), mentionPage.Parent.DatabaseID)
						if err != nil {
							return fmt.Errorf("failed to find db %s.  error: %w", mentionPage.Parent.DatabaseID, err)
						}
						dbTitle := extractPlainTextFromRichText(dbPage.Title)

						childPath := path.Join(dbTitle, fmt.Sprintf("%s.md", pageTitle))
						emptyList := map[string]bool{}
						if err = fetchAndSaveToObsidianVault(client, mentionPage, emptyList, emptyList, path.Join(*obsidianVault, childPath)); err != nil {
							return err
						}
					}

					buffer.WriteString("[[")
					buffer.WriteString(pageTitle)
					buffer.WriteString("]]")

					mentionCache.Set(text.Mention.Page.ID, pageTitle)
				}
			case notion.MentionTypeDatabase:
				value := text.PlainText
				buffer.WriteString("[[")
				buffer.WriteString(value)
				buffer.WriteString("]]")
			case notion.MentionTypeDate:
				buffer.WriteString("[[")
				buffer.WriteString(text.Mention.Date.Start.Format("2006-01-02"))
				buffer.WriteString("]]")
			case notion.MentionTypeLinkPreview:
				buffer.WriteString(fmt.Sprintf("![](%s)", text.Mention.LinkPreview.URL))
			case notion.MentionTypeTemplateMention:
			case notion.MentionTypeUser:
			}
		case notion.RichTextTypeEquation:
			buffer.WriteString(fmt.Sprintf("$%s$", text.Equation.Expression))
		}
	}

	return nil
}

func extractPlainTextFromRichText(richText []notion.RichText) string {
	buffer := new(strings.Builder)

	for _, text := range richText {
		buffer.WriteString(text.PlainText)
	}

	return buffer.String()
}
