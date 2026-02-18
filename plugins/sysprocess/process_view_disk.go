package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"omo/pkg/ui"
)

// diskEntry holds a directory entry with its size for the ncdu-like view
type diskEntry struct {
	name     string
	path     string
	size     int64
	isDir    bool
	isParent bool
}

// newDiskView creates the disk usage (ncdu-like) CoreView
func (pv *ProcessView) newDiskView() *ui.CoreView {
	cv := ui.NewCoreView(pv.app, "Disk Usage (ncdu)")
	cv.SetTableHeaders([]string{"Size", "Name"})
	cv.SetRefreshCallback(pv.fetchDiskData)
	cv.SetActionCallback(pv.handleAction)

	cv.AddKeyBinding("P", "Processes", nil)
	cv.AddKeyBinding("D", "Disk", nil)
	cv.AddKeyBinding("Enter", "Open", nil)
	cv.AddKeyBinding("U", "Up", nil)

	cv.RegisterHandlers()

	// Enter drills down; we handle via rowSelected in handleAction
	// U = go up - add to keyhandler via action callback

	return cv
}

// fetchDiskData returns the current directory's entries with sizes
func (pv *ProcessView) fetchDiskData() ([][]string, error) {
	pv.diskMu.Lock()
	entries := pv.diskEntries
	scanning := pv.diskScanning
	pv.diskMu.Unlock()

	if scanning {
		return [][]string{
			{"", "[yellow]Scanning... [gray](calculating sizes)[white]"},
		}, nil
	}

	if len(entries) == 0 {
		return [][]string{
			{"", "[yellow]No entries[white]"},
		}, nil
	}

	var data [][]string
	for _, e := range entries {
		name := e.name
		if e.isParent {
			name = "[yellow]..[white]"
		} else if e.isDir {
			name = name + "/"
		}

		sizeStr := "-"
		if !e.isParent {
			sizeStr = formatBytes(uint64(e.size))
		}

		data = append(data, []string{sizeStr, name})
	}

	return data, nil
}

// startDiskScan kicks off background scan of current disk path
func (pv *ProcessView) startDiskScan(path string) {
	pv.diskMu.Lock()
	if pv.diskScanning {
		pv.diskMu.Unlock()
		return
	}
	pv.diskPath = path
	pv.diskScanning = true
	pv.diskMu.Unlock()

	go func() {
		entries := pv.scanDirectory(path)
		pv.diskMu.Lock()
		pv.diskEntries = entries
		pv.diskScanning = false
		pv.diskMu.Unlock()

		pv.app.QueueUpdateDraw(func() {
			pv.diskView.RefreshData()
			pv.diskView.SetInfoText(fmt.Sprintf(
				"[yellow]Disk Usage[white]\nPath: %s\nEntries: %d",
				truncateString(path, 50),
				len(entries),
			))
		})
	}()
}

// scanDirectory lists path's contents with sizes, sorted by size desc
func (pv *ProcessView) scanDirectory(path string) []diskEntry {
	var entries []diskEntry

	// Add parent link if not at root
	if path != "/" {
		parent := filepath.Dir(path)
		entries = append(entries, diskEntry{
			name:     "..",
			path:     parent,
			isDir:    true,
			isParent: true,
		})
	}

	dir, err := os.Open(path)
	if err != nil {
		return entries
	}
	defer dir.Close()

	names, err := dir.Readdirnames(-1)
	if err != nil {
		return entries
	}

	type sized struct {
		entry diskEntry
		size  int64
	}
	var sizedList []sized

	for _, name := range names {
		if name == "." || name == ".." {
			continue
		}
		fullPath := filepath.Join(path, name)
		info, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}

		var size int64
		if info.IsDir() {
			size = dirSize(fullPath)
		} else {
			size = info.Size()
		}

		sizedList = append(sizedList, sized{
			entry: diskEntry{
				name:  name,
				path:  fullPath,
				size:  size,
				isDir: info.IsDir(),
			},
			size: size,
		})
	}

	// Sort by size descending
	sort.Slice(sizedList, func(i, j int) bool { return sizedList[i].size > sizedList[j].size })

	for _, s := range sizedList {
		entries = append(entries, s.entry)
	}

	return entries
}

// dirSize returns total size of a directory (walks the tree)
func dirSize(path string) int64 {
	var total int64
	filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

// onDiskRowActivated handles Enter on a disk row — drill down or up
func (pv *ProcessView) onDiskRowActivated(row int) {
	pv.diskMu.Lock()
	entries := pv.diskEntries
	pv.diskMu.Unlock()

	if row < 0 || row >= len(entries) {
		return
	}

	e := entries[row]
	if !e.isDir {
		return
	}

	pv.diskView.Log(fmt.Sprintf("[blue]Opening %s", e.path))
	pv.startDiskScan(e.path)
}

// diskGoUp navigates to parent directory
func (pv *ProcessView) diskGoUp() {
	pv.diskMu.Lock()
	path := pv.diskPath
	pv.diskMu.Unlock()

	if path == "/" {
		pv.diskView.Log("[yellow]Already at root")
		return
	}

	parent := filepath.Dir(path)
	pv.diskView.Log(fmt.Sprintf("[blue]Going up to %s", parent))
	pv.startDiskScan(parent)
}
