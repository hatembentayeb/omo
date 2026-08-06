package pluginrpc

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"omo/pkg/pluginapi"
)

var (
	rpcLogMu   sync.Mutex
	rpcLogFile *os.File
)

// OpenRPCLog appends to ~/.omo/logs/<name>.log for RPC diagnostics.
func OpenRPCLog(name string) error {
	dir := pluginapi.LogsDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, name+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	rpcLogMu.Lock()
	if rpcLogFile != nil {
		_ = rpcLogFile.Close()
	}
	rpcLogFile = f
	line := fmt.Sprintf("%s ==== rpc log opened path=%s pid=%d ====\n",
		time.Now().Format("15:04:05.000"), path, os.Getpid())
	_, _ = rpcLogFile.WriteString(line)
	_ = rpcLogFile.Sync()
	rpcLogMu.Unlock()
	return nil
}

// RPCLog writes a timestamped line to the current RPC log (and stderr if no file).
func RPCLog(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("%s %s\n", time.Now().Format("15:04:05.000"), msg)

	rpcLogMu.Lock()
	defer rpcLogMu.Unlock()
	if rpcLogFile != nil {
		_, _ = rpcLogFile.WriteString(line)
		_ = rpcLogFile.Sync()
		return
	}
	_, _ = fmt.Fprint(os.Stderr, line)
}
