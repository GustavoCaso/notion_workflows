package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/GustavoCaso/notion_workflows/pkg/utils"
	"github.com/dstotijn/go-notion"
)

const (
	dailyCheckDatabaseID = "3b27a5d9-138b-4f50-9c7b-7a77224f0579"
	obsidianVault        = "/Users/gustavocaso/Documents/Obsidian Vault/Thoughts/Personal Notes"
)

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

	result, _ := client.QueryDatabase(context.Background(), dailyCheckDatabaseID, query)
	wg := new(sync.WaitGroup)

	for _, result := range result.Results {
		wg.Add(1)
		properties := result.Properties.(notion.DatabasePageProperties)
		date := properties["Date"].Date.Start
		dayOfTheWeek := &properties["Day of the Week"].Formula.String
		fileName := fmt.Sprintf("%s-%s.md", date.Format("2006-01-02"), **dayOfTheWeek)
		obsidianPath := path.Join(obsidianVault, fmt.Sprint(date.Year()), fmt.Sprintf("%d-%s", int(date.Month()), date.Month().String()), fileName)
		go fetchAndSaveToObsidianVault(wg, client, result.ID, obsidianPath)
	}

	wg.Wait()
}

func fetchAndSaveToObsidianVault(wg *sync.WaitGroup, client *notion.Client, id, path string) {
	defer wg.Done()
	blocks, _ := client.FindBlockChildrenByID(context.Background(), id, nil)

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

	pageToMarkdown(client, blocks, buffer)
}

func pageToMarkdown(client *notion.Client, pageBlocks notion.BlockChildrenResponse, buffer *bufio.Writer) {
	for _, object := range pageBlocks.Results {
		switch block := object.(type) {
		case *notion.Heading3Block:
			buffer.WriteString("### ")
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("\n")
			buffer.WriteString("\n")
		case *notion.ToDoBlock:
			if *block.Checked {
				buffer.WriteString("[x] ")
			} else {
				buffer.WriteString("[ ] ")
			}
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("\n")
			buffer.WriteString("\n")
		case *notion.ParagraphBlock:
			if len(block.RichText) > 0 {
				writeRichText(client, buffer, block.RichText)
				buffer.WriteString("\n")
				buffer.WriteString("\n")
			} else {
				buffer.WriteString("\n")
			}
		case *notion.BulletedListItemBlock:
			buffer.WriteString("- ")
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("\n")
			buffer.WriteString("\n")
		case *notion.NumberedListItemBlock:
			buffer.WriteString("- ")
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("\n")
			buffer.WriteString("\n")
		case *notion.CalloutBlock:
			buffer.WriteString("> [!")
			if len(*block.Icon.Emoji) > 0 {
				buffer.WriteString(*block.Icon.Emoji)
			}
			writeRichText(client, buffer, block.RichText)
			buffer.WriteString("]")
			buffer.WriteString("\n")
			buffer.WriteString("\n")
		default:
			errMessage := fmt.Sprintf("block not supported: %+v", block)
			panic(errMessage)
		}
	}

	if err := buffer.Flush(); err != nil {
		panic(err)
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
				mentionPage, _ := client.FindPageByID(context.Background(), text.Mention.Page.ID)
				if mentionPage.Parent.Type == notion.ParentTypeDatabase {
					props := mentionPage.Properties.(notion.DatabasePageProperties)
					buffer.WriteString("[[")
					writeRichText(client, buffer, props["Name"].Title)
					buffer.WriteString("]]")
				} else if mentionPage.Parent.Type == "" {
				} else {
					errMessage := fmt.Sprintf("mention page not supported: %s", mentionPage.Parent.Type)
					panic(errMessage)
				}
			default:
				panic(fmt.Sprintf("mention type no supported: %s", text.Mention.Type))
			}
		case notion.RichTextTypeEquation:
			panic("rich text equation not supported")
		}
	}
}
