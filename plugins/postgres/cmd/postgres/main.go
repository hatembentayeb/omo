package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/postgres"
)

func main() {
	pluginrpc.Serve(postgres.NewService())
}
