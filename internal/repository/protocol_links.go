package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ProtocolLinks asks every registered protocol for its links.
type ProtocolLinks struct {
	pool  *pgxpool.Pool
	links []Links
}

func NewProtocolLinks(pool *pgxpool.Pool, links []Links) *ProtocolLinks {
	return &ProtocolLinks{pool: pool, links: links}
}

func (this *ProtocolLinks) ParticipantLinks(ctx context.Context, participantID int64) []Link {
	var links []Link
	for _, protocol := range this.links {
		links = append(links, protocol.ParticipantLinks(ctx, this.pool, participantID)...)
	}

	return links
}

func (this *ProtocolLinks) CounterpartURL(ctx context.Context, name string) (string, bool) {
	for _, protocol := range this.links {
		if url, found := protocol.CounterpartURL(ctx, this.pool, name); found {
			return url, true
		}
	}

	return "", false
}
