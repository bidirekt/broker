package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const findParticipantPublishedByNameQuery = `
	SELECT
		EXISTS (
			SELECT
				1
			FROM
				latest_contract_versions l
			WHERE
				l.participant_id = p.id
		)
	FROM
		participants p
	WHERE
		p.name = $1
`

func (this *ParticipantRepository) FindPublishedByName(ctx context.Context, name string) (published, found bool) {
	err := this.pool.QueryRow(ctx, findParticipantPublishedByNameQuery, name).Scan(&published)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding whether the participant has published: %w", err))
	}

	return published, true
}
