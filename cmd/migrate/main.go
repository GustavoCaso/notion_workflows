package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/GustavoCaso/notion_workflows/pkg/utils"
	"github.com/dstotijn/go-notion"
)

const (
	pageID    = "7378e119-fc86-4e0c-b48c-922a14053dd4"
	pageIDBis = "fd5f406e-72b2-4fc1-8b54-f8efc8d893e2"
)

func main() {
	client := notion.NewClient(utils.GetAuthenticationToken())

	result, _ := client.FindBlockChildrenByID(context.Background(), pageIDBis, nil)

	// marshalled, _ := json.Marshal(result)
	// fmt.Println(string(marshalled))

	var markdownResult strings.Builder

	for _, object := range result.Results {
		switch block := object.(type) {
		case *notion.Heading3Block:
			markdownResult.WriteString("### ")
			markdownResult.WriteString(writeRichText(client, block.RichText))
			markdownResult.WriteString("\n")
			markdownResult.WriteString("\n")
		case *notion.ToDoBlock:
			if *block.Checked {
				markdownResult.WriteString("[x] ")
			} else {
				markdownResult.WriteString("[ ] ")
			}
			markdownResult.WriteString(writeRichText(client, block.RichText))
			markdownResult.WriteString("\n")
			markdownResult.WriteString("\n")
		case *notion.ParagraphBlock:
			if len(block.RichText) > 0 {
				markdownResult.WriteString(writeRichText(client, block.RichText))
				markdownResult.WriteString("\n")
				markdownResult.WriteString("\n")
			} else {
				markdownResult.WriteString("\n")
			}
		case *notion.BulletedListItemBlock:
			markdownResult.WriteString("- ")
			markdownResult.WriteString(writeRichText(client, block.RichText))
			markdownResult.WriteString("\n")
			markdownResult.WriteString("\n")
		case *notion.NumberedListItemBlock:
			markdownResult.WriteString("- ")
			markdownResult.WriteString(writeRichText(client, block.RichText))
			markdownResult.WriteString("\n")
			markdownResult.WriteString("\n")
		case *notion.CalloutBlock:
			markdownResult.WriteString("> [!")
			if len(*block.Icon.Emoji) > 0 {
				markdownResult.WriteString(*block.Icon.Emoji)
			}
			markdownResult.WriteString(writeRichText(client, block.RichText))
			markdownResult.WriteString("]")
			markdownResult.WriteString("\n")
			markdownResult.WriteString("\n")
		default:
			panic("type unknown")
		}
	}

	fmt.Println("Result")
	fmt.Print(markdownResult.String())
}

func writeRichText(client *notion.Client, richText []notion.RichText) string {
	var result strings.Builder

	for _, text := range richText {
		switch text.Type {
		case notion.RichTextTypeText:
			if text.Annotations.Color == notion.ColorDefault {
				result.WriteString(text.Text.Content)
			} else {
				result.WriteString("==")
				result.WriteString(text.Text.Content)
				result.WriteString("==")
			}
		case notion.RichTextTypeMention:
			switch text.Mention.Type {
			case notion.MentionTypePage:
				mentionPage, _ := client.FindPageByID(context.Background(), text.Mention.Page.ID)
				if mentionPage.Parent.Type == notion.ParentTypeDatabase {
					props := mentionPage.Properties.(notion.DatabasePageProperties)
					result.WriteString(fmt.Sprintf("[[%s]]", writeRichText(client, props["Name"].Title)))
				} else {
					panic("mention page not supoprted")
				}
			default:
				panic(fmt.Sprintf("mention type no supported: %s", text.Mention.Type))
			}
		case notion.RichTextTypeEquation:
			panic("rich text equation not supported")
		}
	}

	return result.String()
}
