// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
package ui

import (
	"fmt"
	"time"
)

// SetRefreshCallback sets a function to be called when refresh is triggered.
// This callback is responsible for fetching fresh data to display in the table.
// The callback should return updated table data and an optional error.
//
// Parameters:
//   - callback: A function that returns table data ([][]string) and an error
//
// Returns:
//   - The Cores instance for method chaining
func (c *Cores) SetRefreshCallback(callback func() ([][]string, error)) *Cores {
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
//   - The Cores instance for method chaining
func (c *Cores) StartAutoRefresh(interval time.Duration) *Cores {
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
//   - The Cores instance for method chaining
func (c *Cores) StopAutoRefresh() *Cores {
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
// This function calls the refresh callback to fetch fresh data,
// updates the table with the new data, and logs the result.
// It can be called manually or is triggered automatically by
// the refresh timer if auto-refresh is enabled.
//
// Returns:
//   - The Cores instance for method chaining
func (c *Cores) RefreshData() *Cores {
	if c.onRefresh != nil {
		c.Log("Refreshing data...")
		data, err := c.onRefresh()
		if err != nil {
			c.Log(fmt.Sprintf("[red]Error refreshing data: %v", err))
		} else {
			c.SetTableData(data)
			c.Log("[green]Data refreshed successfully")
		}
	}
	return c
}
