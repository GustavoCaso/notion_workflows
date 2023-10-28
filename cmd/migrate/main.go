package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/GustavoCaso/notion_workflows/pkg/utils"
	"github.com/dstotijn/go-notion"
)

const (
	dailyCheckDatabaseID      = "3b27a5d9-138b-4f50-9c7b-7a77224f0579"
	obsidianVault             = "/Users/gustavocaso/Documents/Obsidian Vault/Thoughts/Personal Notes"
	obsidianVaultToCategorize = "/Users/gustavocaso/Documents/Obsidian Vault/Thoughts/Personal Notes/Categorize"
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

func main() {
	client := notion.NewClient(utils.GetAuthenticationToken())

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

	notionResponse, err := client.QueryDatabase(context.Background(), dailyCheckDatabaseID, query)
	if err != nil {
		panic(err)
	}

	wg := new(sync.WaitGroup)

	for _, page := range notionResponse.Results {
		wg.Add(1)
		path := personalNotesPath(page)
		go fetchAndSaveToObsidianVault(wg, client, page, path)
	}

	for notionResponse.HasMore {
		time.Sleep(10 * time.Second)

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
			go fetchAndSaveToObsidianVault(wg, client, page, path)
		}
	}

	wg.Wait()
}

func fetchAndSaveToObsidianVault(wg *sync.WaitGroup, client *notion.Client, page notion.Page, obsidianPath string) {
	defer wg.Done()

	pageBlocks, err := client.FindBlockChildrenByID(context.Background(), page.ID, nil)
	if err != nil {
		fmt.Println("failed to extact blocks when retriveing daily check page")
		panic(err)
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
	buffer.WriteString("Emotions: ")

	for _, emotion := range props["Emotions"].MultiSelect {
		saveToObsidianVault(path.Join(obsidianVaultToCategorize, fmt.Sprintf("%s.md", emotion.Name)))
		buffer.WriteString(fmt.Sprintf("[[%s]] ", emotion.Name))
	}

	buffer.WriteString("\n\n")

	pageToMarkdown(client, pageBlocks.Results, buffer, false)

	if err := buffer.Flush(); err != nil {
		panic(err)
	}
}

func saveToObsidianVault(obsidianPath string) {
	if err := os.MkdirAll(filepath.Dir(obsidianPath), 0770); err != nil {
		panic(err)
	}

	f, err := os.Create(obsidianPath)
	if err != nil {
		panic(err)
	}

	defer f.Close()
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
			writeChrildren(client, object, buffer)
			buffer.WriteString("\n")
		case *notion.Heading2Block:
			if indent {
				buffer.WriteString("	## ")
			} else {
				buffer.WriteString("## ")
			}
			writeRichText(client, buffer, block.RichText)
			writeChrildren(client, object, buffer)
			buffer.WriteString("\n")
		case *notion.Heading3Block:
			if indent {
				buffer.WriteString("	### ")
			} else {
				buffer.WriteString("### ")
			}
			writeRichText(client, buffer, block.RichText)
			writeChrildren(client, object, buffer)
			buffer.WriteString("\n")
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
			writeChrildren(client, object, buffer)
			buffer.WriteString("\n")
		case *notion.ParagraphBlock:
			if len(block.RichText) > 0 {
				if indent {
					buffer.WriteString("	")
					writeRichText(client, buffer, block.RichText)
				} else {
					writeRichText(client, buffer, block.RichText)
				}
			}
			writeChrildren(client, object, buffer)
			buffer.WriteString("\n")
		case *notion.BulletedListItemBlock:
			if indent {
				buffer.WriteString("	- ")
			} else {
				buffer.WriteString("- ")
			}
			writeRichText(client, buffer, block.RichText)
			writeChrildren(client, object, buffer)
			buffer.WriteString("\n")
		case *notion.NumberedListItemBlock:
			if indent {
				buffer.WriteString("	- ")
			} else {
				buffer.WriteString("- ")
			}
			writeRichText(client, buffer, block.RichText)
			writeChrildren(client, object, buffer)
			buffer.WriteString("\n")
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
			buffer.WriteString("\n")
		case *notion.QuoteBlock:
			if indent {
				buffer.WriteString("	> ")
			} else {
				buffer.WriteString("> ")
			}
			writeRichText(client, buffer, block.RichText)
			writeChrildren(client, object, buffer)
			buffer.WriteString("\n")
		case *notion.FileBlock:
		case *notion.DividerBlock:
		case *notion.ChildPageBlock:
		case *notion.CodeBlock:
			buffer.WriteString("```")
			buffer.WriteString(*block.Language)
			buffer.WriteString("\n")
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("```")
			buffer.WriteString("\n")
		case *notion.ImageBlock:
			if block.Type == notion.FileTypeExternal {
				buffer.WriteString(fmt.Sprintf("![](%s)", block.External.URL))
			}
		case *notion.VideoBlock:
			if block.Type == notion.FileTypeExternal {
				buffer.WriteString(fmt.Sprintf("![](%s)", block.External.URL))
			}
		case *notion.EmbedBlock:
			buffer.WriteString(fmt.Sprintf("![](%s)", block.URL))
		case *notion.BookmarkBlock:
			buffer.WriteString(fmt.Sprintf("![](%s)", block.URL))
		default:
			errMessage := fmt.Sprintf("block not supported: %+v", block)
			panic(errMessage)
		}
	}
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
					mentionPage, err := client.FindPageByID(context.Background(), text.Mention.Page.ID)
					if err != nil {
						panic(err)
					}

					var titleRichText []notion.RichText
					if mentionPage.Parent.Type == notion.ParentTypeDatabase {
						props := mentionPage.Properties.(notion.DatabasePageProperties)
						titleRichText = props["Name"].Title
					} else if mentionPage.Parent.Type == notion.ParentTypePage {
						props := mentionPage.Properties.(notion.PageProperties)
						titleRichText = props.Title.Title
					} else if mentionPage.Parent.Type == notion.ParentTypeBlock {
						fmt.Printf("check what this block id is: %s\n", mentionPage.Parent.BlockID)
					} else if mentionPage.Parent.Type == "" {
						fmt.Printf("empty parent type for page %s\n", text.Mention.Page.ID)
					} else {
						errMessage := fmt.Sprintf("mention page not supported: %s", mentionPage.Parent.Type)
						panic(errMessage)
					}

					title := extractRichText(titleRichText)

					if len(title) > 0 {
						fetchBlocksAndSaveToObsidian(client, mentionPage.ID, path.Join(obsidianVaultToCategorize, fmt.Sprintf("%s.md", title)))

						buffer.WriteString("[[")
						buffer.WriteString(title)
						buffer.WriteString("]]")
					} else {
						fmt.Printf("empty title for page id: %s\n", mentionPage.ID)
						saveToObsidianVault(path.Join(obsidianVaultToCategorize, "undefined.md"))
						buffer.WriteString("[[undefined]]")
					}

					mentionCache.Set(text.Mention.Page.ID, title)
				}
			case notion.MentionTypeDatabase:
				datadasePage, err := client.FindDatabaseByID(context.Background(), text.Mention.Database.ID)
				if err != nil {
					panic(err)
				}
				fmt.Printf("Need to figure out what to do with this DB: %s\n", datadasePage.URL)
			case notion.MentionTypeDate:
				buffer.WriteString("[[")
				buffer.WriteString(text.Mention.Date.Start.Format("2006-01-02"))
				buffer.WriteString("]]")
			case notion.MentionTypeLinkPreview:
				buffer.WriteString(fmt.Sprintf("![](%s)", text.Mention.LinkPreview.URL))
			default:
				panic(fmt.Sprintf("mention type no supported: %s", text.Mention.Type))
			}
		case notion.RichTextTypeEquation:
			buffer.WriteString(fmt.Sprintf("$%s$", text.Equation.Expression))
		}
	}
}

func fetchBlocksAndSaveToObsidian(client *notion.Client, id, path string) {
	pageBlocks, err := client.FindBlockChildrenByID(context.Background(), id, nil)
	if err != nil {
		fmt.Printf("failed to extact blocks when retriveing page id: %s\n", id)
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0770); err != nil {
		panic(err)
	}

	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	// create new buffer
	buffer := bufio.NewWriter(f)

	pageToMarkdown(client, pageBlocks.Results, buffer, false)

	if err := buffer.Flush(); err != nil {
		panic(err)
	}
}

func extractRichText(richText []notion.RichText) string {
	buffer := new(strings.Builder)

	for _, text := range richText {
		switch text.Type {
		case notion.RichTextTypeText:
			buffer.WriteString(text.Text.Content)
		default:
			fmt.Printf("do not support extract rich text value for this type: %s\n", text.Type)
		}
	}

	return buffer.String()
}

func personalNotesPath(page notion.Page) string {
	properties := page.Properties.(notion.DatabasePageProperties)
	date := properties["Date"].Date.Start
	dayOfTheWeek := &properties["Day of the Week"].Formula.String
	fileName := fmt.Sprintf("%s-%s.md", date.Format("2006-01-02"), **dayOfTheWeek)
	return path.Join(obsidianVault, fmt.Sprint(date.Year()), fmt.Sprintf("%d-%s", int(date.Month()), date.Month().String()), fileName)
}
