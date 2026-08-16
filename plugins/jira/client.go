package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"omo/pkg/pluginapi"
)

// Client talks to Jira Cloud REST (api v3 + agile + dev-status).
type Client struct {
	site  string
	email string
	token string
	http  *http.Client
	user  *User
}

func NewClient(site, email, token string) *Client {
	return &Client{
		site:  strings.TrimRight(site, "/"),
		email: email,
		token: token,
		http:  pluginapi.NewHTTPClient(30 * time.Second),
	}
}

func (c *Client) Connected() bool {
	return c != nil && c.site != "" && c.email != "" && c.token != ""
}

func (c *Client) Site() string {
	if c == nil {
		return ""
	}
	return c.site
}

func (c *Client) User() *User {
	if c == nil {
		return nil
	}
	return c.user
}

func (c *Client) Myself() (*User, error) {
	var u User
	if err := c.doJSON(http.MethodGet, "/rest/api/3/myself", nil, &u); err != nil {
		return nil, err
	}
	c.user = &u
	return &u, nil
}

func (c *Client) Search(jql string, max int) ([]Issue, error) {
	if max <= 0 {
		max = maxList
	}
	body := map[string]any{
		"jql":        jql,
		"maxResults": max,
		"fields":     issueFields,
	}
	var page issuePage
	if err := c.doJSON(http.MethodPost, "/rest/api/3/search/jql", body, &page); err != nil {
		if isNotFound(err) || isGone(err) {
			if err2 := c.doJSON(http.MethodPost, "/rest/api/3/search", body, &page); err2 != nil {
				return nil, err
			}
			return page.Issues, nil
		}
		return nil, err
	}
	return page.Issues, nil
}

func (c *Client) GetIssue(key string) (*Issue, error) {
	q := url.Values{"fields": {strings.Join(issueFields, ",")}}
	var issue Issue
	if err := c.doJSON(http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(key)+"?"+q.Encode(), nil, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

func (c *Client) CreateIssue(projectKey, issueType, summary string) (*createdIssue, error) {
	body := map[string]any{
		"fields": map[string]any{
			"project":   map[string]any{"key": projectKey},
			"summary":   summary,
			"issuetype": map[string]any{"name": issueType},
		},
	}
	var out createdIssue
	if err := c.doJSON(http.MethodPost, "/rest/api/3/issue", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateMetaTypes(projectKey string) ([]IssueType, error) {
	var out createMetaTypes
	path := "/rest/api/3/issue/createmeta/" + url.PathEscape(projectKey) + "/issuetypes"
	if err := c.doJSON(http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.Values, nil
}

func (c *Client) Transitions(key string) ([]Transition, error) {
	var page transitionsPage
	if err := c.doJSON(http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(key)+"/transitions", nil, &page); err != nil {
		return nil, err
	}
	return page.Transitions, nil
}

func (c *Client) DoTransition(key, transitionID string) error {
	body := map[string]any{"transition": map[string]any{"id": transitionID}}
	return c.doJSON(http.MethodPost, "/rest/api/3/issue/"+url.PathEscape(key)+"/transitions", body, nil)
}

func (c *Client) Assign(key, accountID string) error {
	var body any
	if accountID == "" {
		body = map[string]any{"accountId": nil}
	} else {
		body = map[string]any{"accountId": accountID}
	}
	return c.doJSON(http.MethodPut, "/rest/api/3/issue/"+url.PathEscape(key)+"/assignee", body, nil)
}

func (c *Client) Comments(key string) ([]Comment, error) {
	q := url.Values{"maxResults": {strconv.Itoa(maxList)}, "orderBy": {"-created"}}
	var page commentsPage
	if err := c.doJSON(http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(key)+"/comment?"+q.Encode(), nil, &page); err != nil {
		return nil, err
	}
	return page.Comments, nil
}

func (c *Client) AddComment(key, text string) error {
	body := map[string]any{"body": textToADF(text)}
	return c.doJSON(http.MethodPost, "/rest/api/3/issue/"+url.PathEscape(key)+"/comment", body, nil)
}

func (c *Client) MyFilters() ([]SavedFilter, error) {
	q := url.Values{"expand": {"jql"}, "maxResults": {strconv.Itoa(maxList)}}
	var out []SavedFilter
	if err := c.doJSON(http.MethodGet, "/rest/api/3/filter/my?"+q.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetFilter(id string) (*SavedFilter, error) {
	q := url.Values{"expand": {"jql"}}
	var out SavedFilter
	if err := c.doJSON(http.MethodGet, "/rest/api/3/filter/"+url.PathEscape(id)+"?"+q.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) Boards() ([]Board, error) {
	var all []Board
	start := 0
	for i := 0; i < 5; i++ {
		q := url.Values{
			"startAt":    {strconv.Itoa(start)},
			"maxResults": {strconv.Itoa(maxList)},
		}
		var page agilePage[Board]
		if err := c.doJSON(http.MethodGet, "/rest/agile/1.0/board?"+q.Encode(), nil, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Values...)
		if page.IsLast || len(page.Values) == 0 {
			break
		}
		start += len(page.Values)
	}
	return all, nil
}

func (c *Client) BoardIssues(boardID int64) ([]Issue, error) {
	q := url.Values{
		"maxResults": {strconv.Itoa(maxList)},
		"fields":     {strings.Join(issueFields, ",")},
	}
	var page issuePage
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/issue?%s", boardID, q.Encode())
	if err := c.doJSON(http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return page.Issues, nil
}

func (c *Client) Backlog(boardID int64) ([]Issue, error) {
	q := url.Values{
		"maxResults": {strconv.Itoa(maxList)},
		"fields":     {strings.Join(issueFields, ",")},
	}
	var page issuePage
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/backlog?%s", boardID, q.Encode())
	if err := c.doJSON(http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return page.Issues, nil
}

func (c *Client) Sprints(boardID int64) ([]Sprint, error) {
	q := url.Values{
		"maxResults": {strconv.Itoa(maxList)},
		"state":      {"active,future,closed"},
	}
	var page agilePage[Sprint]
	path := fmt.Sprintf("/rest/agile/1.0/board/%d/sprint?%s", boardID, q.Encode())
	if err := c.doJSON(http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return page.Values, nil
}

func (c *Client) SprintIssues(sprintID int64) ([]Issue, error) {
	q := url.Values{
		"maxResults": {strconv.Itoa(maxList)},
		"fields":     {strings.Join(issueFields, ",")},
	}
	var page issuePage
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue?%s", sprintID, q.Encode())
	if err := c.doJSON(http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return page.Issues, nil
}

func (c *Client) MoveToSprint(sprintID int64, keys []string) error {
	body := map[string]any{"issues": keys}
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue", sprintID)
	return c.doJSON(http.MethodPost, path, body, nil)
}

func (c *Client) DevItems(issueID, issueKey string) ([]DevItem, error) {
	id := strings.TrimSpace(issueID)
	if id == "" {
		issue, err := c.GetIssue(issueKey)
		if err != nil {
			return nil, err
		}
		id = issue.ID
	}
	summary, err := c.devSummary(id)
	if err != nil {
		if isNotFound(err) || isNotAllowed(err) {
			return nil, nil
		}
		return nil, err
	}
	items := []DevItem{}
	seen := map[string]bool{}
	for _, pair := range summary.pairs() {
		detail, err := c.devDetail(id, pair.app, pair.data)
		if err != nil {
			continue
		}
		for _, it := range detail {
			k := it.Kind + "|" + it.Name + "|" + it.URL
			if seen[k] {
				continue
			}
			seen[k] = true
			items = append(items, it)
		}
	}
	return items, nil
}

var issueFields = []string{
	"summary", "status", "issuetype", "priority", "assignee", "updated", "project", "description",
}

type httpError struct {
	method string
	path   string
	status int
	body   string
}

func (e *httpError) Error() string {
	msg := strings.TrimSpace(e.body)
	if msg == "" {
		msg = http.StatusText(e.status)
	}
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	return fmt.Sprintf("jira %s %s: %d %s", e.method, e.path, e.status, msg)
}

func isNotFound(err error) bool {
	e, ok := err.(*httpError)
	return ok && e.status == 404
}

func isGone(err error) bool {
	e, ok := err.(*httpError)
	return ok && e.status == 410
}

func isNotAllowed(err error) bool {
	e, ok := err.(*httpError)
	return ok && (e.status == 401 || e.status == 403)
}

func (c *Client) doJSON(method, path string, body any, out any) error {
	data, err := c.doRaw(method, path, body)
	if err != nil {
		return err
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func (c *Client) doRaw(method, path string, body any) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.site+path, rdr)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.email, c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &httpError{method: method, path: path, status: resp.StatusCode, body: jiraErrMessage(data, resp.Status)}
	}
	return data, nil
}

func jiraErrMessage(data []byte, fallback string) string {
	var wrap struct {
		Message       string            `json:"message"`
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	if json.Unmarshal(data, &wrap) == nil {
		if wrap.Message != "" {
			return wrap.Message
		}
		if len(wrap.ErrorMessages) > 0 {
			return strings.Join(wrap.ErrorMessages, "; ")
		}
		if len(wrap.Errors) > 0 {
			var parts []string
			for k, v := range wrap.Errors {
				parts = append(parts, k+": "+v)
			}
			return strings.Join(parts, "; ")
		}
	}
	s := strings.TrimSpace(string(data))
	if s == "" {
		return fallback
	}
	return s
}
