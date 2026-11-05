package show_check

import (
	"github.com/bidirekt/broker/internal/components/breakreport"
	"github.com/bidirekt/broker/internal/model"
)

const (
	checkIDParam = "id"

	showCheckTemplate = "show_check.html"
)

type ShowCheckPage struct {
	Check      *model.CompatibilityCheck
	Report     breakreport.Report
	Breaking   []breakreport.Counterpart
	Compatible []breakreport.Counterpart
}

func newShowCheckPage(check *model.CompatibilityCheck, report breakreport.Report) ShowCheckPage {
	page := ShowCheckPage{Check: check, Report: report}
	for _, counterpart := range report.Counterparts {
		if counterpart.Breaks > 0 {
			page.Breaking = append(page.Breaking, counterpart)
		} else {
			page.Compatible = append(page.Compatible, counterpart)
		}
	}

	return page
}
