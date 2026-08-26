package gui

import "strings"

func stringRowsEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func stringRowsExtend(current, next []string) bool {
	return len(next) > len(current) && stringRowsEqual(current, next[:len(current)])
}

func preservedRowPosition(current, next []string, selected uint) (uint, bool) {
	if int(selected) >= len(current) {
		return 0, false
	}
	key := tableRowKey(current[selected])
	for position, row := range next {
		if tableRowKey(row) == key {
			return uint(position), true
		}
	}
	return 0, false
}

func tableRowKey(row string) string {
	key, _, _ := strings.Cut(row, "\t")
	return key
}
