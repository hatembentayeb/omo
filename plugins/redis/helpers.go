package redis

import (
	"fmt"
	"sort"
	"strings"
)

func parseKeyspace(infoMap map[string]string) string {
	dbFields := make([]string, 0)
	for key, value := range infoMap {
		if strings.HasPrefix(key, "db") {
			dbFields = append(dbFields, fmt.Sprintf("%s=%s", key, value))
		}
	}
	sort.Strings(dbFields)
	return strings.Join(dbFields, ", ")
}
