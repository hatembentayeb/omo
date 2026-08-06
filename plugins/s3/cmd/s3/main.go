package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/s3"
)

func main() {
	pluginrpc.Serve(s3.NewService())
}
