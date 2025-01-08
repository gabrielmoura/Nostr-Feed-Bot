package util

import (
	"Nostr-feed-bot/internal/db"
	"regexp"
	"strings"
)

func CheckMapFeedEmpty(m db.FeedData) bool {
	if m != nil && len(m) > 0 {
		return true
	}
	return false
}

func ToSnakeCase(str string) string {
	// Regex que permite letras Unicode (\p{L}) e números (\p{N})

	snakeCaseRegex := regexp.MustCompile(`[^\p{L}\p{N}]+`)
	str = snakeCaseRegex.ReplaceAllString(str, "_")
	return strings.Trim(strings.ToLower(strings.TrimSpace(str)), "_")
}

func TernaryString(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
