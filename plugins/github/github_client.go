package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"omo/pkg/pluginapi"
)

type GitHubClient struct {
	account    *GitHubAccount
	activeRepo *GitHubRepo
	httpClient *http.Client
	logger     func(string)
}

func NewGitHubClient() *GitHubClient {
	return &GitHubClient{
		httpClient: pluginapi.NewHTTPClient(15 * time.Second),
	}
}

func (c *GitHubClient) SetLogger(fn func(string)) {
	c.logger = fn
}

func (c *GitHubClient) log(msg string) {
	if c.logger != nil {
		c.logger(msg)
	}
}

func (c *GitHubClient) SetAccount(acct *GitHubAccount) {
	c.account = acct
	c.activeRepo = nil

	// For personal accounts, resolve the owner from the token
	if acct.AccountType != "org" && acct.Owner == "" {
		if login, err := c.GetAuthenticatedUser(); err == nil {
			acct.Owner = login
		}
	}
}

func (c *GitHubClient) GetAuthenticatedUser() (string, error) {
	url := fmt.Sprintf("%s/user", c.baseURL())
	data, err := c.doRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	var user struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(data, &user); err != nil {
		return "", err
	}
	return user.Login, nil
}

func (c *GitHubClient) SetActiveRepo(repo *GitHubRepo) {
	c.activeRepo = repo
}

func (c *GitHubClient) HasAccount() bool {
	return c.account != nil && c.account.Token != ""
}

func (c *GitHubClient) HasActiveRepo() bool {
	return c.HasAccount() && c.activeRepo != nil
}

func (c *GitHubClient) baseURL() string {
	if c.account != nil && c.account.APIURL != "" {
		return strings.TrimRight(c.account.APIURL, "/")
	}
	return "https://api.github.com"
}

func (c *GitHubClient) repoURL() string {
	if c.activeRepo == nil {
		return ""
	}
	return fmt.Sprintf("%s/repos/%s", c.baseURL(), c.activeRepo.FullName)
}

func (c *GitHubClient) doRequest(method, url string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.account.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

// --- Repositories ---

func (c *GitHubClient) ListRepos() ([]GitHubRepo, error) {
	var allRepos []GitHubRepo
	page := 1

	for {
		var url string
		if c.account.AccountType == "org" {
			url = fmt.Sprintf("%s/orgs/%s/repos?per_page=100&page=%d&sort=updated&direction=desc", c.baseURL(), c.account.Owner, page)
		} else {
			url = fmt.Sprintf("%s/user/repos?per_page=100&page=%d&sort=updated&direction=desc&affiliation=owner,collaborator,organization_member", c.baseURL(), page)
		}

		data, err := c.doRequest("GET", url, nil)
		if err != nil {
			return allRepos, err
		}

		var raw []struct {
			Name     string `json:"name"`
			FullName string `json:"full_name"`
			Owner    struct {
				Login string `json:"login"`
			} `json:"owner"`
			Description   *string `json:"description"`
			DefaultBranch string  `json:"default_branch"`
			Private       bool    `json:"private"`
			Fork          bool    `json:"fork"`
			Archived      bool    `json:"archived"`
			Stars         int     `json:"stargazers_count"`
			Language      *string `json:"language"`
			UpdatedAt     string  `json:"updated_at"`
		}

		if err := json.Unmarshal(data, &raw); err != nil {
			return allRepos, fmt.Errorf("parse repos: %w", err)
		}

		if len(raw) == 0 {
			break
		}

		for _, r := range raw {
			desc := ""
			if r.Description != nil {
				desc = *r.Description
			}
			lang := ""
			if r.Language != nil {
				lang = *r.Language
			}
			allRepos = append(allRepos, GitHubRepo{
				Name:          r.Name,
				FullName:      r.FullName,
				Owner:         r.Owner.Login,
				Description:   desc,
				DefaultBranch: r.DefaultBranch,
				Private:       r.Private,
				Fork:          r.Fork,
				Archived:      r.Archived,
				Stars:         r.Stars,
				Language:      lang,
				UpdatedAt:     r.UpdatedAt,
			})
		}

		if len(raw) < 100 {
			break
		}
		page++
	}

	return allRepos, nil
}

// --- Pull Requests ---

func (c *GitHubClient) ListPullRequests(state string) ([]PullRequest, error) {
	url := fmt.Sprintf("%s/pulls?state=%s&per_page=100&sort=updated&direction=desc", c.repoURL(), state)
	data, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		Draft  bool   `json:"draft"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		RequestedReviewers []struct {
			Login string `json:"login"`
		} `json:"requested_reviewers"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		HTMLURL   string    `json:"html_url"`
		Additions int       `json:"additions"`
		Deletions int       `json:"deletions"`
		Comments  int       `json:"comments"`
		Commits   int       `json:"commits"`
		Mergeable *bool     `json:"mergeable"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse PRs: %w", err)
	}

	prs := make([]PullRequest, len(raw))
	for i, r := range raw {
		labels := make([]string, len(r.Labels))
		for j, l := range r.Labels {
			labels[j] = l.Name
		}
		reviewers := make([]string, len(r.RequestedReviewers))
		for j, rv := range r.RequestedReviewers {
			reviewers[j] = rv.Login
		}
		mergeable := "unknown"
		if r.Mergeable != nil {
			if *r.Mergeable {
				mergeable = "yes"
			} else {
				mergeable = "no"
			}
		}
		prs[i] = PullRequest{
			Number:    r.Number,
			Title:     r.Title,
			State:     r.State,
			Author:    r.User.Login,
			Branch:    r.Head.Ref,
			Base:      r.Base.Ref,
			Draft:     r.Draft,
			Mergeable: mergeable,
			Labels:    labels,
			Reviewers: reviewers,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
			URL:       r.HTMLURL,
			Additions: r.Additions,
			Deletions: r.Deletions,
			Comments:  r.Comments,
			Commits:   r.Commits,
		}
	}

	return prs, nil
}

func (c *GitHubClient) GetPullRequest(number int) (*PullRequest, error) {
	url := fmt.Sprintf("%s/pulls/%d", c.repoURL(), number)
	data, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var r struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		Draft  bool   `json:"draft"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		RequestedReviewers []struct {
			Login string `json:"login"`
		} `json:"requested_reviewers"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		HTMLURL   string    `json:"html_url"`
		Additions int       `json:"additions"`
		Deletions int       `json:"deletions"`
		Comments  int       `json:"comments"`
		Commits   int       `json:"commits"`
		Mergeable *bool     `json:"mergeable"`
	}

	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parse PR: %w", err)
	}

	labels := make([]string, len(r.Labels))
	for j, l := range r.Labels {
		labels[j] = l.Name
	}
	reviewers := make([]string, len(r.RequestedReviewers))
	for j, rv := range r.RequestedReviewers {
		reviewers[j] = rv.Login
	}
	mergeable := "unknown"
	if r.Mergeable != nil {
		if *r.Mergeable {
			mergeable = "yes"
		} else {
			mergeable = "no"
		}
	}

	return &PullRequest{
		Number:    r.Number,
		Title:     r.Title,
		State:     r.State,
		Author:    r.User.Login,
		Branch:    r.Head.Ref,
		Base:      r.Base.Ref,
		Draft:     r.Draft,
		Mergeable: mergeable,
		Labels:    labels,
		Reviewers: reviewers,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
		URL:       r.HTMLURL,
		Additions: r.Additions,
		Deletions: r.Deletions,
		Comments:  r.Comments,
		Commits:   r.Commits,
	}, nil
}

func (c *GitHubClient) MergePullRequest(number int, method string) error {
	url := fmt.Sprintf("%s/pulls/%d/merge", c.repoURL(), number)
	body := fmt.Sprintf(`{"merge_method":"%s"}`, method)
	_, err := c.doRequest("PUT", url, strings.NewReader(body))
	return err
}

func (c *GitHubClient) ClosePullRequest(number int) error {
	url := fmt.Sprintf("%s/pulls/%d", c.repoURL(), number)
	body := `{"state":"closed"}`
	_, err := c.doRequest("PATCH", url, strings.NewReader(body))
	return err
}

func (c *GitHubClient) ReopenPullRequest(number int) error {
	url := fmt.Sprintf("%s/pulls/%d", c.repoURL(), number)
	body := `{"state":"open"}`
	_, err := c.doRequest("PATCH", url, strings.NewReader(body))
	return err
}

func (c *GitHubClient) ApprovePullRequest(number int) error {
	url := fmt.Sprintf("%s/pulls/%d/reviews", c.repoURL(), number)
	body := `{"event":"APPROVE"}`
	_, err := c.doRequest("POST", url, strings.NewReader(body))
	return err
}

func (c *GitHubClient) GetPRChecks(number int) ([]PRCheck, error) {
	prData, err := c.doRequest("GET", fmt.Sprintf("%s/pulls/%d", c.repoURL(), number), nil)
	if err != nil {
		return nil, err
	}

	var pr struct {
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	}
	if err := json.Unmarshal(prData, &pr); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/commits/%s/check-runs?per_page=100", c.repoURL(), pr.Head.SHA)
	data, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		CheckRuns []struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			HTMLURL    string `json:"html_url"`
		} `json:"check_runs"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	checks := make([]PRCheck, len(result.CheckRuns))
	for i, cr := range result.CheckRuns {
		checks[i] = PRCheck{
			Name:       cr.Name,
			Status:     cr.Status,
			Conclusion: cr.Conclusion,
			URL:        cr.HTMLURL,
		}
	}
	return checks, nil
}

func (c *GitHubClient) GetPRReviews(number int) ([]PRReview, error) {
	url := fmt.Sprintf("%s/pulls/%d/reviews?per_page=100", c.repoURL(), number)
	data, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		State       string    `json:"state"`
		SubmittedAt time.Time `json:"submitted_at"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	reviews := make([]PRReview, len(raw))
	for i, r := range raw {
		reviews[i] = PRReview{
			ID:        r.ID,
			User:      r.User.Login,
			State:     r.State,
			Body:      r.Body,
			CreatedAt: r.SubmittedAt,
		}
	}
	return reviews, nil
}

// --- Workflows ---

func (c *GitHubClient) ListWorkflows() ([]Workflow, error) {
	url := fmt.Sprintf("%s/actions/workflows?per_page=100", c.repoURL())
	data, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Workflows []struct {
			ID        int64     `json:"id"`
			Name      string    `json:"name"`
			Path      string    `json:"path"`
			State     string    `json:"state"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
			HTMLURL   string    `json:"html_url"`
			BadgeURL  string    `json:"badge_url"`
		} `json:"workflows"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	workflows := make([]Workflow, len(result.Workflows))
	for i, w := range result.Workflows {
		workflows[i] = Workflow{
			ID:        w.ID,
			Name:      w.Name,
			Path:      w.Path,
			State:     w.State,
			CreatedAt: w.CreatedAt,
			UpdatedAt: w.UpdatedAt,
			URL:       w.HTMLURL,
			BadgeURL:  w.BadgeURL,
		}
	}
	return workflows, nil
}

// --- Workflow Runs ---

func (c *GitHubClient) ListWorkflowRuns(status string) ([]WorkflowRun, error) {
	url := fmt.Sprintf("%s/actions/runs?per_page=50&sort=created&direction=desc", c.repoURL())
	if status != "" {
		url += "&status=" + status
	}
	data, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		WorkflowRuns []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			HeadBranch string `json:"head_branch"`
			Event      string `json:"event"`
			Actor      struct {
				Login string `json:"login"`
			} `json:"actor"`
			CreatedAt  time.Time `json:"created_at"`
			UpdatedAt  time.Time `json:"updated_at"`
			HTMLURL    string    `json:"html_url"`
			RunNumber  int       `json:"run_number"`
			RunAttempt int       `json:"run_attempt"`
			WorkflowID int64     `json:"workflow_id"`
		} `json:"workflow_runs"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	workflowMap := make(map[int64]string)
	workflows, _ := c.ListWorkflows()
	for _, w := range workflows {
		workflowMap[w.ID] = w.Name
	}

	runs := make([]WorkflowRun, len(result.WorkflowRuns))
	for i, r := range result.WorkflowRuns {
		duration := ""
		if !r.UpdatedAt.IsZero() && !r.CreatedAt.IsZero() {
			duration = formatDuration(r.UpdatedAt.Sub(r.CreatedAt))
		}
		wfName := r.Name
		if name, ok := workflowMap[r.WorkflowID]; ok {
			wfName = name
		}
		runs[i] = WorkflowRun{
			ID:           r.ID,
			Name:         r.Name,
			WorkflowName: wfName,
			Status:       r.Status,
			Conclusion:   r.Conclusion,
			Branch:       r.HeadBranch,
			Event:        r.Event,
			Actor:        r.Actor.Login,
			Duration:     duration,
			CreatedAt:    r.CreatedAt,
			UpdatedAt:    r.UpdatedAt,
			URL:          r.HTMLURL,
			RunNumber:    r.RunNumber,
			Attempt:      r.RunAttempt,
		}
	}

	return runs, nil
}

func (c *GitHubClient) RerunWorkflow(runID int64) error {
	url := fmt.Sprintf("%s/actions/runs/%d/rerun", c.repoURL(), runID)
	_, err := c.doRequest("POST", url, nil)
	return err
}

func (c *GitHubClient) CancelWorkflowRun(runID int64) error {
	url := fmt.Sprintf("%s/actions/runs/%d/cancel", c.repoURL(), runID)
	_, err := c.doRequest("POST", url, nil)
	return err
}

func (c *GitHubClient) GetWorkflowRunJobs(runID int64) ([]WorkflowRunJob, error) {
	url := fmt.Sprintf("%s/actions/runs/%d/jobs?per_page=100", c.repoURL(), runID)
	data, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Jobs []struct {
			ID          int64     `json:"id"`
			Name        string    `json:"name"`
			Status      string    `json:"status"`
			Conclusion  string    `json:"conclusion"`
			StartedAt   time.Time `json:"started_at"`
			CompletedAt time.Time `json:"completed_at"`
			RunnerName  string    `json:"runner_name"`
		} `json:"jobs"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	jobs := make([]WorkflowRunJob, len(result.Jobs))
	for i, j := range result.Jobs {
		dur := ""
		if !j.CompletedAt.IsZero() && !j.StartedAt.IsZero() {
			dur = formatDuration(j.CompletedAt.Sub(j.StartedAt))
		}
		jobs[i] = WorkflowRunJob{
			ID:         j.ID,
			Name:       j.Name,
			Status:     j.Status,
			Conclusion: j.Conclusion,
			StartedAt:  j.StartedAt,
			Duration:   dur,
			RunnerName: j.RunnerName,
		}
	}
	return jobs, nil
}

func (c *GitHubClient) TriggerWorkflowDispatch(workflowID int64, ref string) error {
	url := fmt.Sprintf("%s/actions/workflows/%d/dispatches", c.repoURL(), workflowID)
	body := fmt.Sprintf(`{"ref":"%s"}`, ref)
	_, err := c.doRequest("POST", url, strings.NewReader(body))
	return err
}

// --- Environment Variables ---

func (c *GitHubClient) ListRepoVariables() ([]EnvVariable, error) {
	url := fmt.Sprintf("%s/actions/variables?per_page=100", c.repoURL())
	data, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Variables []struct {
			Name      string    `json:"name"`
			Value     string    `json:"value"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"variables"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	vars := make([]EnvVariable, len(result.Variables))
	for i, v := range result.Variables {
		vars[i] = EnvVariable{
			Name:      v.Name,
			Value:     v.Value,
			CreatedAt: v.CreatedAt,
			UpdatedAt: v.UpdatedAt,
		}
	}
	return vars, nil
}

func (c *GitHubClient) CreateRepoVariable(name, value string) error {
	url := fmt.Sprintf("%s/actions/variables", c.repoURL())
	body := fmt.Sprintf(`{"name":"%s","value":"%s"}`, name, value)
	_, err := c.doRequest("POST", url, strings.NewReader(body))
	return err
}

func (c *GitHubClient) UpdateRepoVariable(name, value string) error {
	url := fmt.Sprintf("%s/actions/variables/%s", c.repoURL(), name)
	body := fmt.Sprintf(`{"name":"%s","value":"%s"}`, name, value)
	_, err := c.doRequest("PATCH", url, strings.NewReader(body))
	return err
}

func (c *GitHubClient) DeleteRepoVariable(name string) error {
	url := fmt.Sprintf("%s/actions/variables/%s", c.repoURL(), name)
	_, err := c.doRequest("DELETE", url, nil)
	return err
}

// --- Secrets ---

func (c *GitHubClient) ListRepoSecrets() ([]RepoSecret, error) {
	url := fmt.Sprintf("%s/actions/secrets?per_page=100", c.repoURL())
	data, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Secrets []struct {
			Name      string    `json:"name"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"secrets"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	secrets := make([]RepoSecret, len(result.Secrets))
	for i, s := range result.Secrets {
		secrets[i] = RepoSecret{
			Name:      s.Name,
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
		}
	}
	return secrets, nil
}

func (c *GitHubClient) DeleteRepoSecret(name string) error {
	url := fmt.Sprintf("%s/actions/secrets/%s", c.repoURL(), name)
	_, err := c.doRequest("DELETE", url, nil)
	return err
}

// --- Branches ---

func (c *GitHubClient) ListBranches() ([]Branch, error) {
	url := fmt.Sprintf("%s/branches?per_page=100", c.repoURL())
	data, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
		Protected bool `json:"protected"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	defaultBranch := ""
	if c.activeRepo != nil {
		defaultBranch = c.activeRepo.DefaultBranch
	}

	branches := make([]Branch, len(raw))
	for i, b := range raw {
		branches[i] = Branch{
			Name:      b.Name,
			SHA:       b.Commit.SHA,
			Protected: b.Protected,
			Default:   b.Name == defaultBranch,
		}
	}
	return branches, nil
}

func (c *GitHubClient) DeleteBranch(name string) error {
	url := fmt.Sprintf("%s/git/refs/heads/%s", c.repoURL(), name)
	_, err := c.doRequest("DELETE", url, nil)
	return err
}

// --- Releases ---

func (c *GitHubClient) ListReleases() ([]Release, error) {
	url := fmt.Sprintf("%s/releases?per_page=50", c.repoURL())
	data, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		ID         int64  `json:"id"`
		TagName    string `json:"tag_name"`
		Name       string `json:"name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Author     struct {
			Login string `json:"login"`
		} `json:"author"`
		CreatedAt   time.Time `json:"created_at"`
		PublishedAt time.Time `json:"published_at"`
		HTMLURL     string    `json:"html_url"`
		Body        string    `json:"body"`
		Assets      []struct {
			ID int64 `json:"id"`
		} `json:"assets"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	releases := make([]Release, len(raw))
	for i, r := range raw {
		releases[i] = Release{
			ID:          r.ID,
			TagName:     r.TagName,
			Name:        r.Name,
			Draft:       r.Draft,
			Prerelease:  r.Prerelease,
			Author:      r.Author.Login,
			CreatedAt:   r.CreatedAt,
			PublishedAt: r.PublishedAt,
			URL:         r.HTMLURL,
			Body:        r.Body,
			Assets:      len(r.Assets),
		}
	}
	return releases, nil
}

func (c *GitHubClient) DeleteRelease(id int64) error {
	url := fmt.Sprintf("%s/releases/%d", c.repoURL(), id)
	_, err := c.doRequest("DELETE", url, nil)
	return err
}

// --- Environments ---

func (c *GitHubClient) ListEnvironments() ([]Environment, error) {
	url := fmt.Sprintf("%s/environments", c.repoURL())
	data, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Environments []struct {
			ID        int64     `json:"id"`
			Name      string    `json:"name"`
			HTMLURL   string    `json:"html_url"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"environments"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	envs := make([]Environment, len(result.Environments))
	for i, e := range result.Environments {
		envs[i] = Environment{
			ID:        e.ID,
			Name:      e.Name,
			URL:       e.HTMLURL,
			CreatedAt: e.CreatedAt,
			UpdatedAt: e.UpdatedAt,
		}
	}
	return envs, nil
}

func (c *GitHubClient) ListEnvironmentVariables(envName string) ([]EnvVariable, error) {
	url := fmt.Sprintf("%s/environments/%s/variables?per_page=100", c.repoURL(), envName)
	data, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Variables []struct {
			Name      string    `json:"name"`
			Value     string    `json:"value"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
		} `json:"variables"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	vars := make([]EnvVariable, len(result.Variables))
	for i, v := range result.Variables {
		vars[i] = EnvVariable{
			Name:      v.Name,
			Value:     v.Value,
			CreatedAt: v.CreatedAt,
			UpdatedAt: v.UpdatedAt,
		}
	}
	return vars, nil
}
