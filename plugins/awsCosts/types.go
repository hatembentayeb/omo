package awscosts

import "time"

// CostData is a single cost explorer row used by the RPC service.
type CostData struct {
	service   string
	cost      float64
	date      string
	unit      string
	region    string
	usageType string
	trend     float64
	forecast  float64
	budget    float64
}

func getCostTimeRange(timeRange string) struct{ Start, End time.Time } {
	now := time.Now()
	result := struct{ Start, End time.Time }{
		End: now,
	}

	switch timeRange {
	case "LAST_7_DAYS":
		result.Start = now.AddDate(0, 0, -7)
	case "LAST_30_DAYS":
		result.Start = now.AddDate(0, 0, -30)
	case "LAST_3_MONTHS":
		result.Start = now.AddDate(0, -3, 0)
	case "LAST_6_MONTHS":
		result.Start = now.AddDate(0, -6, 0)
	case "LAST_12_MONTHS":
		result.Start = now.AddDate(-1, 0, 0)
	case "THIS_MONTH":
		result.Start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	default:
		result.Start = now.AddDate(0, 0, -30)
	}

	return result
}
