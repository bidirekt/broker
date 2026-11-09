package repository

import (
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/jackc/pgx/v5"
)

const resourceColumns = `
			pa.id,
			pa.name,
			r.id,
			r.direction,
			r.protocol,
			r.body_role,
			r.consumed_provider,
			r.provider_hash,
			r.consumer_hash,
			rv.change_type,
			bodies.sources,
			bodies.schema_names`

// This struct is used to build the entire contract avoiding N+1 problem
type tableRow struct {
	// Participant
	ParticipantID   int64
	ParticipantName string

	// Contract
	ContractID      int64
	ContractVersion string

	// Resource
	ResourceID                int64
	ResourceDirection         string
	ResourceProtocol          string
	ResourceBodyRole          string
	ResourceConsumedProvider  null.String
	ResourceProviderHash      string
	ResourceConsumerHash      null.String
	ResourceVersionChangeType string

	// Bodies, aligned by index
	BodySources     []string
	BodySchemaNames []null.String
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
			Direction:        model.Direction(this.ResourceDirection),
			Protocol:         model.Protocol(this.ResourceProtocol),
			Body:             model.BodyRole(this.ResourceBodyRole),
			Identity:         identity,
			ConsumedProvider: this.ResourceConsumedProvider,
			Bodies:           this.toResourceBodies(),
		},
		ParticipantID: this.ParticipantID,
		ContractID:    this.ContractID,
		ProviderHash:  this.ResourceProviderHash,
		ConsumerHash:  this.ResourceConsumerHash,
		Removed:       this.ResourceVersionChangeType == string(model.ChangeRemoved),
	}
}

func (this *tableRow) toResourceBodies() []model.ResourceBody {
	bodies := make([]model.ResourceBody, len(this.BodySources))
	for index, source := range this.BodySources {
		bodies[index] = model.ResourceBody{Source: source, SchemaName: this.BodySchemaNames[index]}
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
