package main

import (
	"omo/pkg/pluginrpc"
	awscosts "omo/plugins/awsCosts"
)

func main() {
	pluginrpc.Serve(awscosts.NewService())
}
