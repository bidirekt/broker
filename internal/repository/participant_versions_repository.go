package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	findParticipantVersionsHeaderQuery = `
		SELECT
			p.id,
			p.name,
			latest.version,
			(SELECT COUNT(*) FROM contract_versions WHERE participant_id = p.id),
			(SELECT COUNT(*) FROM deployments WHERE participant_id = p.id),
			(SELECT COUNT(*) FROM compatibility_checks WHERE participant_id = p.id),
			(SELECT COUNT(*) FROM (
				SELECT
					r.consumed_provider
				FROM
					resources r
				JOIN LATERAL (
					SELECT change_type
					FROM resource_versions
					WHERE resource_id = r.id AND contract_id <= latest.contract_id
					ORDER BY contract_id DESC
					LIMIT 1
				) rv ON true
				WHERE
					r.participant_id = p.id
				AND
					r.direction = 'consumes'
				AND
					rv.change_type = 'added'
				UNION
				SELECT
					consumer.name
				FROM
					resources r
				JOIN
					participants consumer ON consumer.id = r.participant_id
				JOIN
					current_deployments cd ON cd.participant_id = r.participant_id
				JOIN LATERAL (
					SELECT change_type
					FROM resource_versions
					WHERE resource_id = r.id AND contract_id <= cd.contract_id
					ORDER BY contract_id DESC
					LIMIT 1
				) rv ON true
				WHERE
					r.direction = 'consumes'
				AND
					r.consumed_provider = p.name
				AND
					rv.change_type = 'added'
			) relations)
		FROM
			participants p
		LEFT JOIN
			latest_contract_versions latest ON latest.participant_id = p.id
		WHERE
			p.name = $1
	`

	listParticipantDeployedNowQuery = `
		SELECT
			e.id, e.name, cd.version
		FROM
			current_deployments cd
		JOIN
			environments e ON e.id = cd.environment_id
		WHERE
			cd.participant_id = $1
		ORDER BY
			e.name COLLATE "C"
	`

	listParticipantPublishedVersionsQuery = `
		SELECT
			cv.version,
			cv.created_at,
			COALESCE(array_agg(e.id ORDER BY e.name COLLATE "C") FILTER (WHERE e.id IS NOT NULL), '{}'),
			COALESCE(array_agg(e.name ORDER BY e.name COLLATE "C") FILTER (WHERE e.id IS NOT NULL), '{}')
		FROM
			contract_versions cv
		LEFT JOIN
			current_deployments cd ON cd.participant_id = cv.participant_id AND cd.version = cv.version
		LEFT JOIN
			environments e ON e.id = cd.environment_id
		WHERE
			cv.participant_id = $1
		GROUP BY
			cv.id
		ORDER BY
			cv.id DESC
		LIMIT $2 OFFSET $3
	`
)

type ParticipantVersionsRepository struct {
	pool *pgxpool.Pool
}

func NewParticipantVersionsRepository(pool *pgxpool.Pool) *ParticipantVersionsRepository {
	return &ParticipantVersionsRepository{pool: pool}
}

func (this *ParticipantVersionsRepository) FindHeader(ctx context.Context, name string) (*model.ParticipantVersionsHeader, bool) {
	header := &model.ParticipantVersionsHeader{}

	err := this.pool.QueryRow(ctx, findParticipantVersionsHeaderQuery, name).Scan(
		&header.Participant.ID,
		&header.Participant.Name,
		&header.LatestVersion,
		&header.VersionCount,
		&header.DeploymentCount,
		&header.CheckCount,
		&header.RelationCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding participant versions header: %w", err))
	}

	header.DeployedNow = this.listDeployedNow(ctx, header.Participant.ID)

	return header, true
}

func (this *ParticipantVersionsRepository) listDeployedNow(ctx context.Context, participantID int64) []model.Deployment {
	rows, err := this.pool.Query(ctx, listParticipantDeployedNowQuery, participantID)
	if err != nil {
		panic(fmt.Errorf("error listing participant deployed now: %w", err))
	}

	deployed, err := pgx.AppendRows(make([]model.Deployment, 0), rows, func(row pgx.CollectableRow) (model.Deployment, error) {
		deployment := model.Deployment{Environment: &model.Environment{}}
		err := row.Scan(&deployment.Environment.ID, &deployment.Environment.Name, &deployment.Version)
		return deployment, err
	})
	if err != nil {
		panic(fmt.Errorf("error scanning participant deployed now: %w", err))
	}

	return deployed
}

func (this *ParticipantVersionsRepository) ListPublishedVersions(ctx context.Context, participantID int64, limit, offset int) []model.PublishedVersion {
	rows, err := this.pool.Query(ctx, listParticipantPublishedVersionsQuery, participantID, limit, offset)
	if err != nil {
		panic(fmt.Errorf("error listing participant published versions: %w", err))
	}

	versions, err := pgx.AppendRows(make([]model.PublishedVersion, 0), rows, func(row pgx.CollectableRow) (model.PublishedVersion, error) {
		var (
			version          model.PublishedVersion
			environmentIDs   []int64
			environmentNames []string
		)
		if err := row.Scan(&version.Version, &version.PublishedAt, &environmentIDs, &environmentNames); err != nil {
			return version, err
		}

		version.DeployedIn = make([]model.Environment, len(environmentIDs))
		for i := range environmentIDs {
			version.DeployedIn[i] = model.Environment{ID: environmentIDs[i], Name: environmentNames[i]}
		}

		return version, nil
	})
	if err != nil {
		panic(fmt.Errorf("error scanning participant published versions: %w", err))
	}

	return versions
}
