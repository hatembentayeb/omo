// Command manifest generates ~/.omo/installed.yaml from index.yaml
// by checking which plugins have a .so file in ~/.omo/plugins/.
// Run after "make all" to keep the manifest in sync with built plugins.
package main

import (
	"fmt"
	"os"

	"omo/pkg/pluginapi"
)

func main() {
	data, err := os.ReadFile("index.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read index.yaml: %v\n", err)
		os.Exit(1)
	}

	index, err := pluginapi.ParseIndex(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot parse index.yaml: %v\n", err)
		os.Exit(1)
	}

	for _, entry := range index.Plugins {
		if pluginapi.IsInstalled(entry.Name) {
			pluginapi.RecordInstalledVersion(entry.Name, entry.Version)
		}
	}
}
