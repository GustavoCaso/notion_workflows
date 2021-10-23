package utils

import (
	"errors"
	"os"
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
