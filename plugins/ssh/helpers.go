package ssh

import "fmt"

func buildSSHArgs(srv SSHServer) []string {
	var args []string

	if srv.Port != 0 && srv.Port != 22 {
		args = append(args, "-p", fmt.Sprintf("%d", srv.Port))
	}

	if srv.KeyPath != "" {
		args = append(args, "-i", srv.KeyPath)
	}

	if srv.JumpHost != "" {
		args = append(args, "-J", srv.JumpHost)
	}

	if srv.ProxyCommand != "" {
		args = append(args, "-o", fmt.Sprintf("ProxyCommand=%s", srv.ProxyCommand))
	}

	if srv.KeepAlive > 0 {
		args = append(args, "-o", fmt.Sprintf("ServerAliveInterval=%d", srv.KeepAlive))
		args = append(args, "-o", "ServerAliveCountMax=3")
	}

	args = append(args, "-o", "StrictHostKeyChecking=no")

	if srv.Password != "" && srv.PrivateKey == "" && srv.KeyPath == "" {
		args = append(args, "-o", "PreferredAuthentications=password")
		args = append(args, "-o", "PubkeyAuthentication=no")
	}

	target := srv.Host
	if srv.User != "" {
		target = srv.User + "@" + srv.Host
	}
	args = append(args, target)

	if srv.StartupCmd != "" {
		args = append(args, "-t", srv.StartupCmd)
	}

	return args
}
