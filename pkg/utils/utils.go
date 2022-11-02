package utils

import (
	"bytes"
	"errors"
	"os"
	"text/template"
)

func GetAuthenticationToken() string {
	value := os.Getenv("MORNING_WORKFLOW_API_TOKEN")
	if value == "" {
		panic(errors.New("The ENV variable MORNING_WORKFLOW_API_TOKEN must be set"))
	}
	return value
}

func Contains(value string, values []string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func ExecuteTemplate(fileName, templateName string, data any) (bytes.Buffer, error) {
	var buf bytes.Buffer
	fileBytes, err := os.ReadFile(fileName)
	if err != nil {
		return buf, err
	}

	t := template.Must(template.New(templateName).Parse(string(fileBytes)))
	err = t.Execute(&buf, data)
	if err != nil {
		return buf, err
	}
	return buf, nil
}
