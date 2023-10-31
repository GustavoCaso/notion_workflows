package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dstotijn/go-notion"
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
var wg = new(sync.WaitGroup)

var token = flag.String("token", os.Getenv("NOTION_TOKEN"), "notion token")
var databaseID = flag.String("id", os.Getenv("NOTION_DATABASE_ID"), "notion database ID to migrate")
var obsidianVault = flag.String("d", os.Getenv("OBSIDIAN_VAULT_PATH"), "Obsidian vault location")

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

	if empty(obsidianVault) {
		flag.Usage()
		fmt.Println("You must provide the obisidian vault path to run the script")
		os.Exit(1)
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

	notionResponse, err := client.QueryDatabase(context.Background(), *databaseID, query)
	if err != nil {
		panic(err)
	}

	for _, page := range notionResponse.Results {
		wg.Add(1)
		path := personalNotesPath(page)
		go fetchAndSaveToObsidianVault(client, page, path)
	}

	for notionResponse.HasMore {
		time.Sleep(20 * time.Second)

		fmt.Printf("more pages \n")
		fmt.Printf("next cursor: %s\n", *notionResponse.NextCursor)

		query.StartCursor = *notionResponse.NextCursor

		notionResponse, err = client.QueryDatabase(context.Background(), dailyCheckDatabaseID, query)
		if err != nil {
			panic(err)
		}

		for _, page := range notionResponse.Results {
			wg.Add(1)
			path := personalNotesPath(page)
			go fetchAndSaveToObsidianVault(client, page, path)
		}
	}

	wg.Wait()
}

func empty(v *string) bool {
	return *v == ""
}

func fetchAndSaveToObsidianVault(client *notion.Client, page notion.Page, obsidianPath string) {
	defer wg.Done()

	pageBlocks, err := client.FindBlockChildrenByID(context.Background(), page.ID, nil)
	if err != nil {
		fmt.Printf("failed to extact blocks when retriveing page. Skipping block with ID: %s\n", page.ID)
		return
	}

	if err := os.MkdirAll(filepath.Dir(obsidianPath), 0770); err != nil {
		panic(err)
	}

	f, err := os.Create(obsidianPath)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	// create new buffer
	buffer := bufio.NewWriter(f)

	props := page.Properties.(notion.DatabasePageProperties)

	propertiesToFrontMatter(props, buffer)

	pageToMarkdown(client, pageBlocks.Results, buffer, false)

	if err := buffer.Flush(); err != nil {
		panic(err)
	}
}

func pageToMarkdown(client *notion.Client, blocks []notion.Block, buffer *bufio.Writer, indent bool) {
	for _, object := range blocks {
		switch block := object.(type) {
		case *notion.Heading1Block:
			if indent {
				buffer.WriteString("	# ")
			} else {
				buffer.WriteString("# ")
			}
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("\n")
			writeChrildren(client, object, buffer)
		case *notion.Heading2Block:
			if indent {
				buffer.WriteString("	## ")
			} else {
				buffer.WriteString("## ")
			}
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("\n")
			writeChrildren(client, object, buffer)
		case *notion.Heading3Block:
			if indent {
				buffer.WriteString("	### ")
			} else {
				buffer.WriteString("### ")
			}
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("\n")
			writeChrildren(client, object, buffer)
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
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("\n")
			writeChrildren(client, object, buffer)
		case *notion.ParagraphBlock:
			if len(block.RichText) > 0 {
				if indent {
					buffer.WriteString("	")
					writeRichText(client, buffer, block.RichText)
				} else {
					writeRichText(client, buffer, block.RichText)
				}
			}
			buffer.WriteString("\n")
			writeChrildren(client, object, buffer)
		case *notion.BulletedListItemBlock:
			if indent {
				buffer.WriteString("	- ")
			} else {
				buffer.WriteString("- ")
			}
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("\n")
			writeChrildren(client, object, buffer)
		case *notion.NumberedListItemBlock:
			if indent {
				buffer.WriteString("	- ")
			} else {
				buffer.WriteString("- ")
			}
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("\n")
			writeChrildren(client, object, buffer)
		case *notion.CalloutBlock:
			if indent {
				buffer.WriteString("	> [!")
			} else {
				buffer.WriteString("> [!")
			}
			if len(*block.Icon.Emoji) > 0 {
				buffer.WriteString(*block.Icon.Emoji)
			}
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("]")
			buffer.WriteString("\n")
		case *notion.ToggleBlock:
			if indent {
				buffer.WriteString("	- ")
			} else {
				buffer.WriteString("- ")
			}
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("\n")
			writeChrildren(client, object, buffer)
		case *notion.QuoteBlock:
			if indent {
				buffer.WriteString("	> ")
			} else {
				buffer.WriteString("> ")
			}
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("\n")
			writeChrildren(client, object, buffer)
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
			writeRichText(client, buffer, block.RichText)
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
			errMessage := fmt.Sprintf("block not supported: %+v", block)
			panic(errMessage)
		}
	}
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

func writeChrildren(client *notion.Client, block notion.Block, buffer *bufio.Writer) {
	if block.HasChildren() {
		pageBlocks, err := client.FindBlockChildrenByID(context.Background(), block.ID(), nil)
		if err != nil {
			fmt.Println("failed to extact children blocks")
			panic(err)
		}
		pageToMarkdown(client, pageBlocks.Results, buffer, true)
	}
}

func writeRichText(client *notion.Client, buffer *bufio.Writer, richText []notion.RichText) {
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
						panic(err)
					}

					if mentionPage.Parent.Type == notion.ParentTypeDatabase {
						dbPage, err := client.FindDatabaseByID(context.Background(), mentionPage.Parent.DatabaseID)
						if err != nil {
							panic(err)
						}
						dbTitle := extractPlainTextFromRichText(dbPage.Title)

						childPath := path.Join(dbTitle, fmt.Sprintf("%s.md", pageTitle))
						wg.Add(1)
						fetchAndSaveToObsidianVault(client, mentionPage, path.Join(*obsidianVault, childPath))
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
}

func extractPlainTextFromRichText(richText []notion.RichText) string {
	buffer := new(strings.Builder)

	for _, text := range richText {
		buffer.WriteString(text.PlainText)
	}

	return buffer.String()
}

func personalNotesPath(page notion.Page) string {
	properties := page.Properties.(notion.DatabasePageProperties)
	date := properties["Date"].Date.Start
	fileName := fmt.Sprintf("%s-%s.md", date.Format("2006-01-02"), date.Weekday())
	return path.Join(*obsidianVault, fmt.Sprint(date.Year()), fmt.Sprintf("%d-%s", int(date.Month()), date.Month().String()), fileName)
}
