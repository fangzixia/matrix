package tasks

import (
	"fmt"
	"strings"
)

func trim(s string) string {
	return strings.TrimSpace(s)
}

func userPart(userInput, defaultTask string) string {
	if t := trim(userInput); t != "" {
		return t
	}
	return defaultTask
}

func filePart(label, filePath, ifMissing string) string {
	if filePath != "" {
		return fmt.Sprintf("%s: %s", label, filePath)
	}
	return fmt.Sprintf("%s: %s", label, ifMissing)
}
