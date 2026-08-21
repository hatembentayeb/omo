package pluginapi

import (
	"context"
	"net"
	"os"
	"runtime"
	"strings"
)

// Termux/Android has no working /etc/resolv.conf for Go's pure-Go resolver
// (it often ends up querying [::1]:53). On other systems libc / resolv.conf
// is enough. When we detect Termux or GOOS=android, send DNS to public
// resolvers instead.
func init() {
	if !needsPublicDNS() {
		return
	}
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial:     dialNameservers([]string{"8.8.8.8:53", "1.1.1.1:53"}),
	}
}

func needsPublicDNS() bool {
	if os.Getenv("TERMUX_VERSION") != "" || os.Getenv("TERMUX_PREFIX") != "" {
		return true
	}
	if runtime.GOOS == "android" {
		return true
	}
	return strings.Contains(os.Getenv("PREFIX"), "com.termux")
}

func dialNameservers(nameservers []string) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, _ string) (net.Conn, error) {
		d := net.Dialer{}
		var last error
		for _, ns := range nameservers {
			conn, err := d.DialContext(ctx, network, ns)
			if err == nil {
				return conn, nil
			}
			last = err
		}
		return nil, last
	}
}
