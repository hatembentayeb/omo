package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/dnscheck"
)

func main() {
	pluginrpc.Serve(dnscheck.NewService())
}
