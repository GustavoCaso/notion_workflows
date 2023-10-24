package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/GustavoCaso/notion_workflows/pkg/utils"
	"github.com/dstotijn/go-notion"
)

const (
	dailyCheckDatabaseID = "3b27a5d9-138b-4f50-9c7b-7a77224f0579"
	pageID               = "7378e119-fc86-4e0c-b48c-922a14053dd4"
)

func main() {
	client := notion.NewClient(utils.GetAuthenticationToken())

	result, _ := client.FindBlockChildrenByID(context.Background(), pageID, nil)

	// marshalled, _ := json.Marshal(result)
	// fmt.Println(string(marshalled))

	var markdownResult strings.Builder

	for _, object := range result.Results {
		switch block := object.(type) {
		case *notion.Heading3Block:
			markdownResult.WriteString("### ")
			markdownResult.WriteString(writeRichText(block.RichText))
			markdownResult.WriteString("\n")
		case *notion.ToDoBlock:
			if *block.Checked {
				markdownResult.WriteString("- [x] ")
			} else {
				markdownResult.WriteString("- [ ] ")
			}
			markdownResult.WriteString(writeRichText(block.RichText))
			markdownResult.WriteString("\n")
		default:
			fmt.Println("type unknown")
		}
	}

	fmt.Println("Result")
	fmt.Print(markdownResult.String())
}

func writeRichText(richText []notion.RichText) string {
	var result strings.Builder

	for _, text := range richText {
		switch text.Type {
		case notion.RichTextTypeText:
			result.WriteString(text.Text.Content)
		case notion.RichTextTypeMention:
			panic("Heading 3 mention not supported")
		case notion.RichTextTypeEquation:
			panic("Heading 3 equation not supported")
		}
	}

	return result.String()
}
