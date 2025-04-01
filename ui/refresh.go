package ui

import (
	"fmt"
	"time"
)

// SetRefreshCallback sets a function to be called when refresh is triggered
func (c *Cores) SetRefreshCallback(callback func() ([][]string, error)) *Cores {
	c.onRefresh = callback
	return c
}

// StartAutoRefresh starts automatic refreshing at the given interval
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

// StopAutoRefresh stops the automatic refresh
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

// RefreshData manually triggers a refresh of the data
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
