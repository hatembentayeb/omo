package jira

import "encoding/json"

type User struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

type Board struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Location struct {
		ProjectID   int64  `json:"projectId"`
		ProjectKey  string `json:"projectKey"`
		ProjectName string `json:"projectName"`
		DisplayName string `json:"displayName"`
	} `json:"location"`
}

func (b Board) ProjectKey() string {
	if b.Location.ProjectKey != "" {
		return b.Location.ProjectKey
	}
	return b.Location.DisplayName
}

type Sprint struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	State     string `json:"state"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type StatusCategory struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type Status struct {
	Name           string         `json:"name"`
	StatusCategory StatusCategory `json:"statusCategory"`
}

type IssueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

type Priority struct {
	Name string `json:"name"`
}

type ProjectRef struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type IssueFields struct {
	Summary     string          `json:"summary"`
	Description json.RawMessage `json:"description"`
	IssueType   IssueType       `json:"issuetype"`
	Status      Status          `json:"status"`
	Priority    *Priority       `json:"priority"`
	Assignee    *User           `json:"assignee"`
	Project     *ProjectRef     `json:"project"`
	Updated     string          `json:"updated"`
}

type Issue struct {
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

func (i Issue) TypeName() string {
	return i.Fields.IssueType.Name
}

func (i Issue) StatusName() string {
	return i.Fields.Status.Name
}

func (i Issue) PriorityName() string {
	if i.Fields.Priority == nil || i.Fields.Priority.Name == "" {
		return "-"
	}
	return i.Fields.Priority.Name
}

func (i Issue) AssigneeName() string {
	if i.Fields.Assignee == nil || i.Fields.Assignee.DisplayName == "" {
		return "Unassigned"
	}
	return i.Fields.Assignee.DisplayName
}

func (i Issue) ProjectKey() string {
	if i.Fields.Project == nil {
		return ""
	}
	return i.Fields.Project.Key
}

type Transition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	To   Status `json:"to"`
}

type Comment struct {
	ID      string          `json:"id"`
	Author  User            `json:"author"`
	Created string          `json:"created"`
	Body    json.RawMessage `json:"body"`
}

type SavedFilter struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	JQL  string `json:"jql"`
}

type selectedIssue struct {
	Key        string
	ID         string
	ProjectKey string
	Summary    string
}

type DevItem struct {
	Kind  string
	Name  string
	Env   string
	State string
	When  string
	URL   string
}

type agilePage[T any] struct {
	MaxResults int  `json:"maxResults"`
	StartAt    int  `json:"startAt"`
	Total      int  `json:"total"`
	IsLast     bool `json:"isLast"`
	Values     []T  `json:"values"`
}

type issuePage struct {
	Issues        []Issue `json:"issues"`
	NextPageToken string  `json:"nextPageToken"`
	Total         int     `json:"total"`
	StartAt       int     `json:"startAt"`
}

type transitionsPage struct {
	Transitions []Transition `json:"transitions"`
}

type commentsPage struct {
	Comments []Comment `json:"comments"`
	Total    int       `json:"total"`
}

type createMetaTypes struct {
	Values []IssueType `json:"values"`
}

type createdIssue struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}
