package main

import (
	"log"
	"os"
	"strconv"
	"strings"
)

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return val
}

func optionalEnv(key string) string {
	return os.Getenv(key)
}

func parseAllowedIDs(raw string) map[int64]bool {
	ids := make(map[int64]bool)
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			log.Fatalf("invalid chat ID %q: %v", s, err)
		}
		ids[id] = true
	}
	return ids
}

func parseChatIDList(raw string) []int64 {
	var ids []int64
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func parseStringSet(raw string) map[string]bool {
	ids := make(map[string]bool)
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		ids[s] = true
	}
	return ids
}

func parseStringList(raw string) []string {
	var ids []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		ids = append(ids, s)
	}
	return ids
}

func splitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}
	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}
		cut := maxLen
		if idx := strings.LastIndex(text[:cut], "\n"); idx > 0 {
			cut = idx + 1
		}
		chunks = append(chunks, strings.TrimRight(text[:cut], "\n"))
		text = text[cut:]
	}
	return chunks
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
