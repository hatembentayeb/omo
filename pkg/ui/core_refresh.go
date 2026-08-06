// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
package ui

import (
	"fmt"
	"time"

	"github.com/rivo/tview"
)

// SetRefreshCallback sets a function to be called when refresh is triggered.
// This callback is responsible for fetching fresh data to display in the table.
// The callback should return updated table data and an optional error.
//
// Parameters:
//   - callback: A function that returns table data ([][]string) and an error
//
// Returns:
//   - The CoreView instance for method chaining
func (c *CoreView) SetRefreshCallback(callback func() ([][]string, error)) *CoreView {
	c.onRefresh = callback
	return c
}

// StartAutoRefresh starts automatic refreshing at the given interval.
// This function creates a background goroutine that periodically triggers
// data refresh based on the specified interval. It ensures that any existing
// refresh timers are properly stopped before starting a new one.
//
// Parameters:
//   - interval: The time duration between automatic refreshes
//
// Returns:
//   - The CoreView instance for method chaining
func (c *CoreView) StartAutoRefresh(interval time.Duration) *CoreView {
	c.refreshMutex.Lock()
	defer c.refreshMutex.Unlock()

	// Stop any existing refresh
	if c.refreshTicker != nil {
		c.StopAutoRefresh()
	}

	c.refreshTicker = time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-c.refreshTicker.C:
				c.RefreshData()
			case <-c.stopRefresh:
				return
			}
		}
	}()

	c.Log(fmt.Sprintf("Auto-refresh enabled (%s)", interval))
	return c
}

// StopAutoRefresh stops the automatic refresh.
// This function halts the background refresh goroutine and cleans up
// associated resources. It's important to call this method when the
// component is no longer needed to prevent resource leaks.
//
// Returns:
//   - The CoreView instance for method chaining
func (c *CoreView) StopAutoRefresh() *CoreView {
	c.refreshMutex.Lock()
	defer c.refreshMutex.Unlock()

	if c.refreshTicker != nil {
		c.refreshTicker.Stop()
		c.refreshTicker = nil
		close(c.stopRefresh)
		c.stopRefresh = make(chan struct{})
		c.Log("Auto-refresh disabled")
	}

	return c
}

// RefreshData manually triggers a refresh of the data.
// The refresh callback runs in a background goroutine; table updates and logs
// are applied on the tview UI thread so the terminal stays responsive.
//
// Returns:
//   - The CoreView instance for method chaining
func (c *CoreView) RefreshData() *CoreView {
	c.dataMutex.Lock()
	if c.isLoading {
		c.dataMutex.Unlock()
		c.Log("[yellow]Loading in progress...")
		return c
	}
	c.isLoading = true
	app := c.app
	c.dataMutex.Unlock()

	if app == nil {
		c.dataMutex.Lock()
		c.isLoading = false
		c.dataMutex.Unlock()
		return c
	}

	if c.lazyLoader != nil {
		go c.runLazyRefreshInitial(app)
		return c
	}
	if c.onRefresh != nil {
		go c.runOnRefresh(app)
		return c
	}

	c.dataMutex.Lock()
	c.isLoading = false
	c.dataMutex.Unlock()
	return c
}

func (c *CoreView) runLazyRefreshInitial(app *tview.Application) {
	loader := c.lazyLoader
	pageSize := c.lazyPageSize
	if loader == nil {
		c.clearLoadingOnUI(app)
		return
	}
	c.Log("Refreshing data...")
	data, err := loader(0, pageSize)
	app.QueueUpdateDraw(func() {
		c.dataMutex.Lock()
		c.isLoading = false
		if err != nil {
			c.dataMutex.Unlock()
			c.Log(fmt.Sprintf("[red]Error refreshing data: %v", err))
			return
		}
		c.lazyOffset = len(data)
		c.lazyHasMore = len(data) >= pageSize
		c.rawTableData = data
		c.tableData = c.applyFilter(data)
		c.refreshTable()
		c.dataMutex.Unlock()
		c.Log("[green]Data refreshed successfully")
	})
}

func (c *CoreView) runOnRefresh(app *tview.Application) {
	cb := c.onRefresh
	if cb == nil {
		c.clearLoadingOnUI(app)
		return
	}
	c.Log("Refreshing data...")
	data, err := cb()
	app.QueueUpdateDraw(func() {
		c.dataMutex.Lock()
		c.isLoading = false
		if err != nil {
			c.dataMutex.Unlock()
			c.Log(fmt.Sprintf("[red]Error refreshing data: %v", err))
			return
		}
		c.rawTableData = data
		c.tableData = c.applyFilter(data)
		c.refreshTable()
		c.dataMutex.Unlock()
		c.Log("[green]Data refreshed successfully")
	})
}

func (c *CoreView) clearLoadingOnUI(app *tview.Application) {
	app.QueueUpdateDraw(func() {
		c.dataMutex.Lock()
		c.isLoading = false
		c.dataMutex.Unlock()
	})
}

// SetLazyLoader enables lazy loading with a page size and loader function.
func (c *CoreView) SetLazyLoader(pageSize int, loader func(offset, limit int) ([][]string, error)) *CoreView {
	if pageSize <= 0 {
		pageSize = 500
	}
	c.lazyPageSize = pageSize
	c.lazyLoader = loader
	c.lazyHasMore = true
	c.keyBindings["PgDn"] = "Load more"
	return c
}

// LoadMore fetches the next page when lazy loading is enabled.
// The loader runs in a background goroutine; UI updates run on the tview thread.
func (c *CoreView) LoadMore() *CoreView {
	if c.lazyLoader == nil {
		return c
	}
	c.dataMutex.Lock()
	if c.isLoading {
		c.dataMutex.Unlock()
		return c
	}
	if !c.lazyHasMore {
		c.dataMutex.Unlock()
		c.Log("[yellow]No more rows to load")
		return c
	}
	c.isLoading = true
	offset := c.lazyOffset
	pageSize := c.lazyPageSize
	loader := c.lazyLoader
	app := c.app
	c.dataMutex.Unlock()

	if app == nil {
		c.dataMutex.Lock()
		c.isLoading = false
		c.dataMutex.Unlock()
		return c
	}

	go func() {
		data, err := loader(offset, pageSize)
		app.QueueUpdateDraw(func() {
			c.finalizeLoadMore(data, err)
		})
	}()
	return c
}

func (c *CoreView) finalizeLoadMore(data [][]string, err error) {
	c.dataMutex.Lock()
	c.isLoading = false
	if err != nil {
		c.dataMutex.Unlock()
		c.Log(fmt.Sprintf("[red]Error loading more: %v", err))
		return
	}
	if len(data) == 0 {
		c.lazyHasMore = false
		c.dataMutex.Unlock()
		c.Log("[yellow]No more rows to load")
		return
	}
	c.rawTableData = append(c.rawTableData, data...)
	c.tableData = c.applyFilter(c.rawTableData)
	c.lazyOffset += len(data)
	if len(data) < c.lazyPageSize {
		c.lazyHasMore = false
	}
	c.refreshTable()
	c.dataMutex.Unlock()
	c.Log(fmt.Sprintf("[green]Loaded %d more rows", len(data)))
}
