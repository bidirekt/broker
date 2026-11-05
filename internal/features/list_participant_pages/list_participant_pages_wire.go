package list_participant_pages

import (
	"github.com/bidirekt/broker/internal/components/pages"
	"github.com/bidirekt/broker/internal/model"
)

const (
	pageParam = "page"

	listParticipantPagesTemplate = "list_participant_pages.html"
)

type ListParticipantPagesPage struct {
	Participants []*model.ParticipantListing
	Pager        pages.Pager
}
