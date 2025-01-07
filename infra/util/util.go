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
	snakeCaseRegex := regexp.MustCompile("[^a-zA-Z0-9]+")
	str = snakeCaseRegex.ReplaceAllString(str, "_")
	return strings.ToLower(strings.TrimSpace(str))
}

func TernaryString(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
