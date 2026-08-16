package jira

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type devSummary struct {
	PullRequest json.RawMessage `json:"pullrequest"`
	Build       json.RawMessage `json:"build"`
	Review      json.RawMessage `json:"review"`
	Repository  json.RawMessage `json:"repository"`
	Branch      json.RawMessage `json:"branch"`
	Deployment  json.RawMessage `json:"deployment"`
}

type devPair struct {
	app  string
	data string
}

func (s devSummary) pairs() []devPair {
	out := []devPair{}
	seen := map[string]bool{}
	add := func(data string, raw json.RawMessage) {
		for _, app := range instanceTypes(raw) {
			k := app + "|" + data
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, devPair{app: app, data: data})
		}
	}
	add("pullrequest", s.PullRequest)
	add("build", s.Build)
	add("review", s.Review)
	add("repository", s.Repository)
	add("branch", s.Branch)
	add("deployment", s.Deployment)
	return out
}

func instanceTypes(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var wrap struct {
		Overall struct {
			Count int `json:"count"`
		} `json:"overall"`
		ByInstanceType map[string]struct {
			Count int    `json:"count"`
			Name  string `json:"name"`
		} `json:"byInstanceType"`
	}
	if json.Unmarshal(raw, &wrap) != nil {
		return nil
	}
	var apps []string
	for k, v := range wrap.ByInstanceType {
		if v.Count > 0 {
			apps = append(apps, k)
		}
	}
	if len(apps) == 0 && wrap.Overall.Count > 0 {
		apps = []string{"GitHub", "bitbucket", "stash"}
	}
	return apps
}

func (c *Client) devSummary(issueID string) (*devSummary, error) {
	q := url.Values{"issueId": {issueID}}
	var out devSummary
	if err := c.doJSON(http.MethodGet, "/rest/dev-status/latest/issue/summary?"+q.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) devDetail(issueID, app, dataType string) ([]DevItem, error) {
	q := url.Values{
		"issueId":         {issueID},
		"applicationType": {app},
		"dataType":        {dataType},
	}
	raw, err := c.doRaw(http.MethodGet, "/rest/dev-status/latest/issue/detail?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	return parseDevDetail(dataType, raw), nil
}

func parseDevDetail(dataType string, raw []byte) []DevItem {
	var wrap struct {
		Detail []map[string]json.RawMessage `json:"detail"`
	}
	if json.Unmarshal(raw, &wrap) != nil {
		return nil
	}
	var items []DevItem
	for _, block := range wrap.Detail {
		switch dataType {
		case "pullrequest":
			items = append(items, parseNamedList(block["pullRequests"], "PR")...)
		case "build":
			items = append(items, parseNamedList(block["builds"], "Build")...)
		case "deployment":
			items = append(items, parseNamedList(block["deployments"], "Deploy")...)
		case "branch":
			items = append(items, parseNamedList(block["branches"], "Branch")...)
		case "repository":
			items = append(items, parseNamedList(block["repositories"], "Repo")...)
		case "review":
			items = append(items, parseNamedList(block["reviews"], "Review")...)
		}
	}
	return items
}

func parseNamedList(raw json.RawMessage, kind string) []DevItem {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var rows []map[string]any
	if json.Unmarshal(raw, &rows) != nil {
		return nil
	}
	out := make([]DevItem, 0, len(rows))
	for _, row := range rows {
		name := firstNonEmpty(asString(row["name"]), asString(row["displayName"]), asString(row["key"]), asString(row["id"]))
		state := firstNonEmpty(asString(row["status"]), asString(row["state"]), asString(row["lastStatus"]))
		env := firstNonEmpty(asString(row["environment"]), nestedString(row["environment"], "name"), nestedString(row["environment"], "displayName"))
		when := firstNonEmpty(asString(row["lastUpdate"]), asString(row["lastUpdated"]), asString(row["lastUpdatedDate"]), asString(row["date"]))
		url := firstNonEmpty(asString(row["url"]), nestedString(row["url"], "url"))
		if name == "" {
			name = "-"
		}
		out = append(out, DevItem{
			Kind:  kind,
			Name:  name,
			Env:   dash(env),
			State: dash(state),
			When:  formatWhen(when),
			URL:   url,
		})
	}
	return out
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	default:
		return ""
	}
}

func nestedString(v any, key string) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	return asString(m[key])
}
