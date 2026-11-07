package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/jackc/pgx/v5"
)

const (
	findRelationsHeaderQuery = `
		SELECT
			p.id,
			p.name,
			l.version,
			(SELECT count(*) FROM contract_versions WHERE participant_id = p.id),
			(SELECT count(*) FROM deployments WHERE participant_id = p.id),
			(SELECT count(*) FROM compatibility_checks WHERE participant_id = p.id)
		FROM
			participants p
			LEFT JOIN latest_contract_versions l ON l.participant_id = p.id
		WHERE
			p.name = $1
	`

	listRelationsDeploymentsQuery = `
		SELECT
			e.id, e.name, cd.version
		FROM
			current_deployments cd
			JOIN environments e ON e.id = cd.environment_id
		WHERE
			cd.participant_id = $1
		ORDER BY
			e.name COLLATE "C"
	`

	listConsumedProvidersQuery = `
		SELECT
			consumed.provider,
			p.id IS NOT NULL,
			EXISTS (SELECT 1 FROM contract_versions WHERE participant_id = p.id),
			e.id,
			e.name,
			cd.version
		FROM (
			SELECT DISTINCT
				r.consumed_provider AS provider
			FROM
				latest_contract_versions l
				JOIN resources r ON r.participant_id = l.participant_id
				JOIN LATERAL (
					SELECT change_type
					FROM resource_versions
					WHERE resource_id = r.id AND contract_id <= l.contract_id
					ORDER BY contract_id DESC
					LIMIT 1
				) rv ON true
			WHERE
				l.participant_id = $1
				AND r.direction = 'consumes'
				AND rv.change_type = 'added'
		) consumed
			LEFT JOIN participants p ON p.name = consumed.provider
			LEFT JOIN current_deployments cd ON cd.participant_id = p.id
			LEFT JOIN environments e ON e.id = cd.environment_id
		ORDER BY
			consumed.provider COLLATE "C", e.name COLLATE "C"
	`

	listConsumersByEnvironmentQuery = `
		SELECT
			e.id, e.name, p.name, cd.version
		FROM
			current_deployments cd
			JOIN environments e ON e.id = cd.environment_id
			JOIN participants p ON p.id = cd.participant_id
		WHERE
			EXISTS (
				SELECT 1
				FROM
					resources r
					JOIN LATERAL (
						SELECT change_type
						FROM resource_versions
						WHERE resource_id = r.id AND contract_id <= cd.contract_id
						ORDER BY contract_id DESC
						LIMIT 1
					) rv ON true
				WHERE
					r.participant_id = cd.participant_id
					AND r.direction = 'consumes'
					AND r.consumed_provider = $1
					AND rv.change_type = 'added'
			)
		ORDER BY
			e.name COLLATE "C", p.name COLLATE "C"
	`
)

func (this *ParticipantRepository) FindRelationsHeader(ctx context.Context, name string) (*model.RelationsHeader, bool) {
	header := &model.RelationsHeader{Participant: &model.Participant{}}

	err := this.pool.QueryRow(ctx, findRelationsHeaderQuery, name).Scan(
		&header.Participant.ID,
		&header.Participant.Name,
		&header.LatestVersion,
		&header.VersionCount,
		&header.DeploymentCount,
		&header.CheckCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		panic(fmt.Errorf("error finding relations header: %w", err))
	}

	rows, err := this.pool.Query(ctx, listRelationsDeploymentsQuery, header.Participant.ID)
	if err != nil {
		panic(fmt.Errorf("error listing relations deployments: %w", err))
	}

	header.Deployed, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Deployment, error) {
		deployment := model.Deployment{Environment: &model.Environment{}}
		err := row.Scan(&deployment.Environment.ID, &deployment.Environment.Name, &deployment.Version)
		return deployment, err
	})
	if err != nil {
		panic(fmt.Errorf("error scanning relations deployments: %w", err))
	}

	return header, true
}

func (this *ParticipantRepository) ListConsumedProviders(ctx context.Context, participantID int64) []model.ConsumedProvider {
	rows, err := this.pool.Query(ctx, listConsumedProvidersQuery, participantID)
	if err != nil {
		panic(fmt.Errorf("error listing consumed providers: %w", err))
	}
	defer rows.Close()

	var providers []model.ConsumedProvider
	for rows.Next() {
		var provider model.ConsumedProvider
		var environmentID null.Int
		var environmentName, version null.String
		if err := rows.Scan(&provider.Name, &provider.Registered, &provider.Published, &environmentID, &environmentName, &version); err != nil {
			panic(fmt.Errorf("error scanning consumed providers: %w", err))
		}

		if len(providers) == 0 || providers[len(providers)-1].Name != provider.Name {
			providers = append(providers, provider)
		}
		if environmentID.Valid {
			last := &providers[len(providers)-1]
			last.Deployed = append(last.Deployed, model.Deployment{Version: version.String, Environment: &model.Environment{ID: environmentID.Int64, Name: environmentName.String}})
		}
	}
	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error scanning consumed providers: %w", err))
	}

	return providers
}

func (this *ParticipantRepository) ListConsumersByEnvironment(ctx context.Context, providerName string) []model.EnvironmentConsumers {
	rows, err := this.pool.Query(ctx, listConsumersByEnvironmentQuery, providerName)
	if err != nil {
		panic(fmt.Errorf("error listing consumers by environment: %w", err))
	}
	defer rows.Close()

	var environments []model.EnvironmentConsumers
	for rows.Next() {
		var environment model.Environment
		consumer := model.Deployment{Participant: &model.Participant{}}
		if err := rows.Scan(&environment.ID, &environment.Name, &consumer.Participant.Name, &consumer.Version); err != nil {
			panic(fmt.Errorf("error scanning consumers by environment: %w", err))
		}

		if len(environments) == 0 || environments[len(environments)-1].Environment.ID != environment.ID {
			environments = append(environments, model.EnvironmentConsumers{Environment: environment})
		}
		last := &environments[len(environments)-1]
		last.Consumers = append(last.Consumers, consumer)
	}
	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error scanning consumers by environment: %w", err))
	}

	return environments
}
