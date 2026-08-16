package jira

const (
	viewMine        = "mine"
	viewBoards      = "boards"
	viewIssues      = "issues"
	viewSprints     = "sprints"
	viewBacklog     = "backlog"
	viewTransitions = "transitions"
	viewComments    = "comments"
	viewDeploys     = "deploys"
	viewFilters     = "filters"
	viewSite        = "site"

	brandName = "Jira Cloud"

	jqlMine      = "assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC"
	jqlWatched   = "watcher = currentUser() AND statusCategory != Done ORDER BY updated DESC"
	jqlReported  = "reporter = currentUser() ORDER BY updated DESC"
	jqlDoneToday = "assignee = currentUser() AND statusCategory = Done AND resolved >= startOfDay() ORDER BY updated DESC"
	jqlDeployed  = "assignee = currentUser() AND development[deployments].environmentType is not EMPTY ORDER BY updated DESC"

	maxList = 50
)
