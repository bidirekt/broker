package pages

import "strconv"

func ParsePositiveInteger(raw string) (int64, bool) {
	value, err := strconv.ParseInt(raw, 10, 64)

	return value, err == nil && value > 0 && strconv.FormatInt(value, 10) == raw
}
