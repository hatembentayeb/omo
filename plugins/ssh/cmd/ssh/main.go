package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/ssh"
)

func main() {
	pluginrpc.Serve(ssh.NewService())
}
