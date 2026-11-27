package repository

import (
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/jackc/pgx/v5"
)

func resourceColumns(changeTypeColumn string) string {
	return `
			pa.id,
			pa.name,
			r.id,
			r.direction,
			r.protocol,
			r.body_role,
			r.consumed_provider,
			r.provider_hash,
			r.consumer_hash,
			` + changeTypeColumn + `,
			bodies.sources,
			bodies.schema_namespaces,
			bodies.schema_names`
}

// contractResourcesJoin keeps a contract without resources as one row whose resource columns are null.
func contractResourcesJoin(changeTypeCondition string) string {
	return `LEFT JOIN LATERAL (
			SELECT r.id, r.direction, r.protocol, r.body_role, r.consumed_provider, r.provider_hash, r.consumer_hash, rv.change_type
			FROM resources r
			JOIN LATERAL (
				SELECT change_type
				FROM resource_versions
				WHERE resource_id = r.id AND contract_id <= c.id
				ORDER BY contract_id DESC
				LIMIT 1
			) rv ON true
			WHERE r.participant_id = c.participant_id` + changeTypeCondition + `
		) r ON true`
}

func resourceBodiesJoin(contractIDColumn string) string {
	return `LEFT JOIN LATERAL (
			SELECT
				array_agg(rb.source ORDER BY rb.source COLLATE "C", rb.id) AS sources,
				array_agg(COALESCE(s.namespace, '') ORDER BY rb.source COLLATE "C", rb.id) AS schema_namespaces,
				array_agg(s.name ORDER BY rb.source COLLATE "C", rb.id) AS schema_names
			FROM resource_bodies rb
			LEFT JOIN schemas s ON s.id = rb.schema_id
			WHERE rb.resource_id = r.id AND rb.contract_id = ` + contractIDColumn + `
		) bodies ON true`
}

// This struct is used to build the entire contract avoiding N+1 problem
type tableRow struct {
	// Participant
	ParticipantID   int64
	ParticipantName string

	// Contract
	ContractID      int64
	ContractVersion string

	// Resource, null on the row of a contract without resources
	ResourceID                null.Int
	ResourceDirection         null.String
	ResourceProtocol          null.String
	ResourceBodyRole          null.String
	ResourceConsumedProvider  null.String
	ResourceProviderHash      null.String
	ResourceConsumerHash      null.String
	ResourceVersionChangeType null.String

	// Bodies, aligned by index
	BodySources          []string
	BodySchemaNamespaces []string
	BodySchemaNames      []null.String
}

func (this *tableRow) toPersistedContractModel() *model.PersistedContract {
	return &model.PersistedContract{
		ID:              this.ContractID,
		ParticipantID:   this.ParticipantID,
		ParticipantName: this.ParticipantName,
		Version:         this.ContractVersion,
		Resources:       make(map[string]model.PersistedResource),
	}
}

func (this *tableRow) toResourceModel(identity model.ResourceIdentity) model.PersistedResource {
	return model.PersistedResource{
		Resource: model.Resource{
			ParticipantName:  this.ParticipantName,
			Direction:        model.Direction(this.ResourceDirection.String),
			Protocol:         model.Protocol(this.ResourceProtocol.String),
			Body:             model.BodyRole(this.ResourceBodyRole.String),
			Identity:         identity,
			ConsumedProvider: this.ResourceConsumedProvider,
			Bodies:           this.toResourceBodies(),
		},
		ParticipantID: this.ParticipantID,
		ContractID:    this.ContractID,
		ProviderHash:  this.ResourceProviderHash.String,
		ConsumerHash:  this.ResourceConsumerHash,
		Removed:       this.ResourceVersionChangeType.String == string(model.ChangeRemoved),
	}
}

func (this *tableRow) toResourceBodies() []model.ResourceBody {
	bodies := make([]model.ResourceBody, len(this.BodySources))
	for index, source := range this.BodySources {
		bodies[index] = model.ResourceBody{Source: source, Namespace: this.BodySchemaNamespaces[index], SchemaName: this.BodySchemaNames[index]}
	}

	return bodies
}

func scanTableRows(rows pgx.Rows) []tableRow {
	defer rows.Close()

	var tableRows []tableRow

	for rows.Next() {
		var row tableRow

		if err := rows.Scan(
			&row.ContractID,
			&row.ContractVersion,
			&row.ParticipantID,
			&row.ParticipantName,
			&row.ResourceID,
			&row.ResourceDirection,
			&row.ResourceProtocol,
			&row.ResourceBodyRole,
			&row.ResourceConsumedProvider,
			&row.ResourceProviderHash,
			&row.ResourceConsumerHash,
			&row.ResourceVersionChangeType,
			&row.BodySources,
			&row.BodySchemaNamespaces,
			&row.BodySchemaNames,
		); err != nil {
			panic(fmt.Errorf("error scanning resource row: %w", err))
		}

		tableRows = append(tableRows, row)
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading resource rows: %w", err))
	}

	return tableRows
}
