package list_participant_pages

import (
	"net/url"

	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
)

const (
	pageParam = "page"

	listParticipantPagesTemplate = "list_participant_pages.html"
)

type ListParticipantPagesPage struct {
	Participants []ParticipantRow
	Pager        pages.Pager
}

type ParticipantRow struct {
	*model.ParticipantListing
	URL string
}

func newListParticipantPagesPage(participants []*model.ParticipantListing, pager pages.Pager) ListParticipantPagesPage {
	page := ListParticipantPagesPage{Participants: make([]ParticipantRow, 0, len(participants)), Pager: pager}
	for _, participant := range participants {
		page.Participants = append(page.Participants, ParticipantRow{ParticipantListing: participant, URL: participantURL(participant.Name)})
	}

	return page
}

func participantURL(participantName string) string {
	return "/participants/" + url.PathEscape(participantName)
}
