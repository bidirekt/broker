package pages

import (
	"fmt"
	"html/template"
	"time"
)

const (
	dateLayout         = "2006-01-02"
	minuteLayout       = "2006-01-02 15:04 UTC"
	day                = 24 * time.Hour
	listRelativeMaxAge = 3
)

func RelativeTime(at, now time.Time) string {
	age := now.Sub(at)
	if age < time.Hour {
		return "just now"
	}

	hours := int(age / time.Hour)
	days := int(age / day)
	switch {
	case days == 0 && hours == 1:
		return "1 hour ago"
	case days == 0:
		return fmt.Sprintf("%d hours ago", hours)
	case days == 1:
		return "yesterday"
	}

	return fmt.Sprintf("%d days ago", days)
}

func ListTime(at, now time.Time) string {
	if int(now.Sub(at)/day) <= listRelativeMaxAge {
		return RelativeTime(at, now)
	}

	return at.UTC().Format(dateLayout)
}

func DetailTime(at, now time.Time) string {
	return at.UTC().Format(minuteLayout) + " · " + RelativeTime(at, now)
}

func ExpiryTime(at time.Time) string {
	return at.UTC().Format(minuteLayout)
}

func MachineTime(at time.Time) string {
	return at.UTC().Format(time.RFC3339Nano)
}

func timeFuncs(now func() time.Time) template.FuncMap {
	return template.FuncMap{
		"listTime":   func(at time.Time) template.HTML { return timeElement(at, ListTime(at, now())) },
		"detailTime": func(at time.Time) template.HTML { return timeElement(at, DetailTime(at, now())) },
		"expiryTime": func(at time.Time) template.HTML { return timeElement(at, ExpiryTime(at)) },
	}
}

func timeElement(at time.Time, text string) template.HTML {
	return template.HTML(`<time class="time" datetime="` + MachineTime(at) + `" title="` + at.UTC().Format(minuteLayout) + `">` + text + `</time>`)
}
