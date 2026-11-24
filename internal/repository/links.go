package repository

import "context"

// Link is a protocol's link to one of its own pages, drawn by the core's pages as it comes.
type Link struct {
	Label string
	URL   string
}

// Links are what a protocol links from the core's pages; it panics on a database error.
type Links interface {
	ParticipantLinks(ctx context.Context, q Querier, participantID int64) []Link
	// CounterpartURL is the page of a counterpart that is not a participant.
	CounterpartURL(ctx context.Context, q Querier, name string) (url string, found bool)
}
