package show_check

import (
	"maps"
	"net/url"
	"slices"

	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/model"
)

const (
	checkIDParam = "id"

	showCheckTemplate = "show_check.html"
)

type ShowCheckPage struct {
	Check          *model.CompatibilityCheck
	ParticipantURL string
	VersionURL     string
	Report         breakreport.Report
	Counterparts   []CounterpartRow
	Breaking       []CounterpartRow
	Compatible     []CounterpartRow
	DrawnSections  []string
}

type CounterpartRow struct {
	breakreport.Counterpart
	URL        string
	VersionURL string
}

func newShowCheckPage(output ShowCheckOutput) ShowCheckPage {
	check, report := output.Check, output.Report
	page := ShowCheckPage{
		Check:          check,
		ParticipantURL: participantURL(check.ParticipantName),
		VersionURL:     versionURL(check.ParticipantName, check.Version),
		Report:         report,
	}
	drawn := map[string]bool{}
	for _, counterpart := range report.Counterparts {
		row := toCounterpartRow(counterpart, output.CounterpartURLs)
		page.Counterparts = append(page.Counterparts, row)
		if counterpart.Breaks > 0 {
			page.Breaking = append(page.Breaking, row)
		} else {
			page.Compatible = append(page.Compatible, row)
		}
		for _, protocol := range counterpart.Protocols {
			drawn[protocol.ReportKey] = true
		}
	}
	page.DrawnSections = slices.Sorted(maps.Keys(drawn))

	return page
}

func toCounterpartRow(counterpart breakreport.Counterpart, counterpartURLs map[string]string) CounterpartRow {
	row := CounterpartRow{Counterpart: counterpart}
	if !counterpart.Registered {
		row.URL = counterpartURLs[counterpart.Name]
		return row
	}

	row.URL = participantURL(counterpart.Name)
	row.VersionURL = versionURL(counterpart.Name, counterpart.Version)

	return row
}

func participantURL(participantName string) string {
	return "/participants/" + url.PathEscape(participantName)
}

func versionURL(participantName, version string) string {
	if version == "" || version == "." || version == ".." {
		return ""
	}

	return participantURL(participantName) + "/versions/" + url.PathEscape(version)
}
