package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/jira"
)

func main() {
	pluginrpc.Serve(jira.NewService())
}
