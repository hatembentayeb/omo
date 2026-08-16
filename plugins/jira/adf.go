package jira

import (
	"encoding/json"
	"strings"
)

type adfNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text"`
	Content []adfNode `json:"content"`
}

func adfToText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var n adfNode
	if err := json.Unmarshal(raw, &n); err != nil {
		return strings.TrimSpace(string(raw))
	}
	return strings.TrimSpace(walkADF(n))
}

func walkADF(n adfNode) string {
	if n.Type == "text" {
		return n.Text
	}
	parts := make([]string, 0, len(n.Content))
	for _, c := range n.Content {
		if s := walkADF(c); s != "" {
			parts = append(parts, s)
		}
	}
	sep := ""
	switch n.Type {
	case "doc", "bulletList", "orderedList":
		sep = "\n"
	}
	return strings.Join(parts, sep)
}

func textToADF(s string) map[string]any {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	parts := strings.Split(s, "\n")
	content := make([]any, 0, len(parts))
	for _, p := range parts {
		para := map[string]any{"type": "paragraph"}
		if strings.TrimSpace(p) != "" {
			para["content"] = []any{
				map[string]any{"type": "text", "text": p},
			}
		}
		content = append(content, para)
	}
	if len(content) == 0 {
		content = []any{map[string]any{"type": "paragraph"}}
	}
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
}
