package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/jackc/pgx/v5"
)

const (
	insertSchemaQuery = `
		INSERT INTO schemas
			(participant_id, name)
		VALUES
			($1, $2)
		RETURNING id
	`

	findSchemaIDByNameQuery = `
		SELECT id FROM schemas WHERE participant_id = $1 AND name = $2
	`

	findSchemaIDsByParticipantQuery = `
		SELECT id, name FROM schemas WHERE participant_id = $1
	`

	insertSchemaVersionQuery = `
		INSERT INTO schema_versions
			(schema_id, contract_id, change_type)
		VALUES
			($1, $2, $3)
	`

	insertFieldQuery = `
		INSERT INTO fields
			(schema_id, path)
		VALUES
			($1, $2)
		RETURNING id
	`

	findFieldIDByPathQuery = `
		SELECT id FROM fields WHERE schema_id = $1 AND path = $2
	`

	insertFieldVersionQuery = `
		INSERT INTO field_versions
			(field_id, contract_id, type, optional, nullable, ref, change_type)
		VALUES
			($1, $2, NULLIF($3, ''), $4, $5, NULLIF($6, ''), $7)
	`

	schemasAtContractQuery = `
		SELECT s.name, f.path, COALESCE(fv.type, ''), fv.optional, fv.nullable, COALESCE(fv.ref, '')
		FROM contracts c
		JOIN schemas s ON s.participant_id = c.participant_id
		JOIN LATERAL (
			SELECT change_type
			FROM schema_versions
			WHERE schema_id = s.id AND contract_id <= c.id
			ORDER BY contract_id DESC
			LIMIT 1
		) sv ON true
		JOIN fields f ON f.schema_id = s.id
		JOIN LATERAL (
			SELECT type, optional, nullable, ref, change_type
			FROM field_versions
			WHERE field_id = f.id AND contract_id <= c.id
			ORDER BY contract_id DESC
			LIMIT 1
		) fv ON true
		WHERE c.id = $1
		  AND sv.change_type = 'added'
		  AND fv.change_type <> 'removed'
	`
)

// schemaCatalogs keeps the catalog live at each anchor snapshot, keyed by contract id, so the reads that share it
// load each catalog once.
type schemaCatalogs map[int64]model.Schemas

func (this *ContractRepository) loadSchemasAt(
	ctx context.Context,
	catalogs schemaCatalogs,
	contractID int64,
) model.Schemas {
	if schemas, loaded := catalogs[contractID]; loaded {
		return schemas
	}

	rows, err := this.pool.Query(ctx, schemasAtContractQuery, contractID)
	if err != nil {
		panic(fmt.Errorf("error loading schemas: %w", err))
	}

	defer rows.Close()

	schemas := model.Schemas{}

	for rows.Next() {
		var name, path string
		var field model.Field

		if err := rows.Scan(&name, &path, &field.Type, &field.Optional, &field.Nullable, &field.Ref); err != nil {
			panic(fmt.Errorf("error scanning schema field: %w", err))
		}

		if schemas[name] == nil {
			schemas[name] = model.Schema{}
		}

		schemas[name][path] = field
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading schemas: %w", err))
	}

	catalogs[contractID] = schemas

	return schemas
}

func (this *ContractRepository) insertAddedSchema(
	ctx context.Context,
	tx pgx.Tx,
	contract *model.UploadedContract,
	name string,
	schema model.Schema,
) {
	schemaID := this.insertSchema(ctx, tx, contract.ParticipantID, name)
	this.insertSchemaVersion(ctx, tx, schemaID, contract.ID, model.ChangeAdded)

	for path, field := range schema {
		fieldID := this.insertField(ctx, tx, schemaID, path)
		this.insertFieldVersion(ctx, tx, fieldID, contract.ID, field, model.ChangeAdded)
	}
}

// insertRemovedSchema also removes every live field, so a schema added back later starts from its new fields only.
func (this *ContractRepository) insertRemovedSchema(
	ctx context.Context,
	tx pgx.Tx,
	contract *model.UploadedContract,
	name string,
	fields map[string]model.FieldChange,
) {
	schemaID, _ := this.findSchemaID(ctx, tx, contract.ParticipantID, name)
	this.insertSchemaVersion(ctx, tx, schemaID, contract.ID, model.ChangeRemoved)

	for path, change := range fields {
		fieldID, _ := this.findFieldID(ctx, tx, schemaID, path)
		this.insertFieldVersion(ctx, tx, fieldID, contract.ID, change.Before, model.ChangeRemoved)
	}
}

func (this *ContractRepository) insertFieldChanges(
	ctx context.Context,
	tx pgx.Tx,
	contract *model.UploadedContract,
	name string,
	fields map[string]model.FieldChange,
) {
	schemaID, _ := this.findSchemaID(ctx, tx, contract.ParticipantID, name)

	for path, change := range fields {
		switch change.Kind {
		case model.ChangeAdded:
			fieldID := this.insertField(ctx, tx, schemaID, path)
			this.insertFieldVersion(ctx, tx, fieldID, contract.ID, change.After, model.ChangeAdded)
		case model.ChangeModified:
			fieldID, _ := this.findFieldID(ctx, tx, schemaID, path)
			this.insertFieldVersion(ctx, tx, fieldID, contract.ID, change.After, model.ChangeModified)
		case model.ChangeRemoved:
			fieldID, _ := this.findFieldID(ctx, tx, schemaID, path)
			this.insertFieldVersion(ctx, tx, fieldID, contract.ID, change.Before, model.ChangeRemoved)
		}
	}
}

func (this *ContractRepository) insertSchema(ctx context.Context, tx pgx.Tx, participantID int64, name string) int64 {
	if id, ok := this.findSchemaID(ctx, tx, participantID, name); ok {
		return id
	}

	var id int64
	if err := tx.QueryRow(ctx, insertSchemaQuery, participantID, name).Scan(&id); err != nil {
		panic(fmt.Errorf("error inserting schema: %w", err))
	}

	return id
}

func (this *ContractRepository) findSchemaID(ctx context.Context, tx pgx.Tx, participantID int64, name string) (int64, bool) {
	return findID(ctx, tx, findSchemaIDByNameQuery, participantID, name)
}

func (this *ContractRepository) findSchemaIDsByName(ctx context.Context, tx pgx.Tx, participantID int64) map[string]int64 {
	rows, err := tx.Query(ctx, findSchemaIDsByParticipantQuery, participantID)
	if err != nil {
		panic(fmt.Errorf("error loading schema ids: %w", err))
	}

	defer rows.Close()

	ids := make(map[string]int64)

	for rows.Next() {
		var id int64
		var name string

		if err := rows.Scan(&id, &name); err != nil {
			panic(fmt.Errorf("error scanning schema id: %w", err))
		}

		ids[name] = id
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading schema ids: %w", err))
	}

	return ids
}

func (this *ContractRepository) insertSchemaVersion(
	ctx context.Context,
	tx pgx.Tx,
	schemaID int64,
	contractID int64,
	kind model.ChangeKind,
) {
	if _, err := tx.Exec(ctx, insertSchemaVersionQuery, schemaID, contractID, string(kind)); err != nil {
		panic(fmt.Errorf("error inserting schema version: %w", err))
	}
}

func (this *ContractRepository) insertField(ctx context.Context, tx pgx.Tx, schemaID int64, path string) int64 {
	if id, ok := this.findFieldID(ctx, tx, schemaID, path); ok {
		return id
	}

	var id int64
	if err := tx.QueryRow(ctx, insertFieldQuery, schemaID, path).Scan(&id); err != nil {
		panic(fmt.Errorf("error inserting field: %w", err))
	}

	return id
}

func (this *ContractRepository) findFieldID(ctx context.Context, tx pgx.Tx, schemaID int64, path string) (int64, bool) {
	return findID(ctx, tx, findFieldIDByPathQuery, schemaID, path)
}

func (this *ContractRepository) insertFieldVersion(
	ctx context.Context,
	tx pgx.Tx,
	fieldID int64,
	contractID int64,
	field model.Field,
	kind model.ChangeKind,
) {
	if _, err := tx.Exec(
		ctx,
		insertFieldVersionQuery,
		fieldID,
		contractID,
		field.Type,
		field.Optional,
		field.Nullable,
		field.Ref,
		string(kind),
	); err != nil {
		panic(fmt.Errorf("error inserting field version: %w", err))
	}
}

func findID(ctx context.Context, tx pgx.Tx, query string, args ...any) (int64, bool) {
	var id int64

	err := tx.QueryRow(ctx, query, args...).Scan(&id)
	if err == nil {
		return id, true
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}

	panic(fmt.Errorf("error looking up id: %w", err))
}
