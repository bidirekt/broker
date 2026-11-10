package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/guregu/null"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	loadContractContentForVersionQuery = `
		SELECT contract_content
		FROM contract_versions
		WHERE participant_id = $1 AND version = $2
	`

	lockContractParticipantQuery = `
		SELECT
			1
		FROM
			participants
		WHERE
			id = $1
		FOR NO KEY UPDATE
	`

	insertContractQuery = `
		INSERT INTO contracts
			(participant_id, checksum)
		VALUES
			($1, $2)
		RETURNING id
	`

	insertContractVersionQuery = `
		INSERT INTO contract_versions
			(participant_id, version, contract_id, contract_content)
		VALUES
			($1, $2, $3, $4)
	`

	aliasVersionToSnapshotQuery = `
		INSERT INTO contract_versions
			(participant_id, version, contract_id, contract_content)
		SELECT $1, $2, id, $4 FROM contracts WHERE participant_id = $1 AND checksum = $3
	`

	insertResourceQuery = `
		INSERT INTO resources
			(
				participant_id,
				direction,
				protocol,
				body_role,
				consumed_provider,
				provider_hash,
				consumer_hash
			)
		VALUES
			($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	findResourceIDByProviderHashQuery = `
		SELECT id FROM resources WHERE direction = 'provides' AND provider_hash = $1
	`

	findResourceIDByConsumerHashQuery = `
		SELECT id FROM resources WHERE direction = 'consumes' AND consumer_hash = $1
	`

	insertResourceBodyQuery = `
		INSERT INTO resource_bodies
			(resource_id, contract_id, source, schema_id)
		VALUES
			($1, $2, $3, $4)
	`

	insertResourceVersionQuery = `
		INSERT INTO resource_versions
			(resource_id, contract_id, change_type)
		VALUES
			($1, $2, $3)
	`

	getLatestSnapshotByParticipantID = `
		SELECT
			c.id,
			(SELECT version FROM contract_versions WHERE contract_id = c.id ORDER BY id DESC LIMIT 1),` + resourceColumns + `
		FROM contracts c
		JOIN participants pa ON pa.id = c.participant_id
		JOIN resources r ON r.participant_id = c.participant_id
		JOIN LATERAL (
			SELECT change_type
			FROM resource_versions
			WHERE resource_id = r.id AND contract_id <= c.id
			ORDER BY contract_id DESC
			LIMIT 1
		) rv ON true
		JOIN LATERAL (
			SELECT
				array_agg(rb.source ORDER BY rb.source COLLATE "C", rb.id) AS sources,
				array_agg(s.name ORDER BY rb.source COLLATE "C", rb.id) AS schema_names
			FROM resource_bodies rb
			LEFT JOIN schemas s ON s.id = rb.schema_id
			WHERE rb.resource_id = r.id AND rb.contract_id = c.id
		) bodies ON true
		WHERE c.participant_id = $1
		  AND c.id = (SELECT MAX(id) FROM contracts WHERE participant_id = pa.id)
		  AND rv.change_type = 'added'
		ORDER BY r.id
	`

	contractVersionTreeQuery = `
		SELECT
			c.id,
			cv.version,` + resourceColumns + `
		FROM contract_versions cv
		JOIN contracts c ON c.id = cv.contract_id
		JOIN participants pa ON pa.id = cv.participant_id
		JOIN resources r ON r.participant_id = cv.participant_id
		JOIN LATERAL (
			SELECT change_type
			FROM resource_versions
			WHERE resource_id = r.id AND contract_id <= c.id
			ORDER BY contract_id DESC
			LIMIT 1
		) rv ON true
		JOIN LATERAL (
			SELECT
				array_agg(rb.source ORDER BY rb.source COLLATE "C", rb.id) AS sources,
				array_agg(s.name ORDER BY rb.source COLLATE "C", rb.id) AS schema_names
			FROM resource_bodies rb
			LEFT JOIN schemas s ON s.id = rb.schema_id
			WHERE rb.resource_id = r.id AND rb.contract_id = c.id
		) bodies ON true
	`

	getContractByNameAndVersion = contractVersionTreeQuery + `
		WHERE pa.name = $1
		  AND cv.version = $2
		ORDER BY r.id
	`

	getContractByIDAndVersion = contractVersionTreeQuery + `
		WHERE cv.contract_id = $1
		  AND cv.version = $2
		ORDER BY r.id
	`
)

type ContractRepository struct {
	pool   *pgxpool.Pool
	tables map[model.Protocol]ProtocolTable
}

func NewContractRepository(pool *pgxpool.Pool, protocols []ProtocolTable) *ContractRepository {
	return &ContractRepository{pool: pool, tables: toProtocolTables(protocols)}
}

func (this *ContractRepository) loadFilesForVersion(ctx context.Context, tx pgx.Tx, participantID int64, version string) ([]model.ContractFile, bool) {
	var contractContent string

	err := tx.QueryRow(ctx, loadContractContentForVersionQuery, participantID, version).Scan(&contractContent)
	if err == nil {
		return toContractFiles(contractContent), true
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	panic(fmt.Errorf("error loading contract content for version: %w", err))
}

func (this *ContractRepository) Publish(
	ctx context.Context,
	contract *model.UploadedContract,
	diff func(*model.PersistedContract) model.ContractDiff,
	sameContent func(stored []model.ContractFile) bool,
) (conflict bool) {
	tx, err := this.pool.Begin(ctx)
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, lockContractParticipantQuery, contract.ParticipantID); err != nil {
		panic(fmt.Errorf("error locking contract participant: %w", err))
	}

	if stored, exists := this.loadFilesForVersion(ctx, tx, contract.ParticipantID, contract.Version); exists {
		return !sameContent(stored)
	}

	if !this.aliasVersionToSnapshot(ctx, tx, contract) {
		if current, exists := this.latestSnapshot(ctx, tx, contract.ParticipantID); exists {
			this.insertNextSnapshot(ctx, tx, contract, current, diff(current))
		} else {
			this.insertFirstSnapshot(ctx, tx, contract)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		panic(fmt.Errorf("error committing transaction: %w", err))
	}

	return false
}

func (this *ContractRepository) insertFirstSnapshot(
	ctx context.Context,
	tx pgx.Tx,
	contract *model.UploadedContract,
) {
	this.insertContract(ctx, tx, contract)
	this.insertContractVersion(ctx, tx, contract)

	for name, schema := range contract.Schemas {
		this.insertAddedSchema(ctx, tx, contract, name, schema)
	}

	for _, resource := range contract.Resources {
		resourceID := this.insertResource(ctx, tx, contract.ParticipantID, &resource)
		this.insertResourceVersion(ctx, tx, resourceID, contract.ID, model.ChangeAdded)
	}

	this.insertResourceBodies(ctx, tx, contract)
}

func (this *ContractRepository) insertNextSnapshot(
	ctx context.Context,
	tx pgx.Tx,
	next *model.UploadedContract,
	current *model.PersistedContract,
	diff model.ContractDiff,
) {
	this.insertContract(ctx, tx, next)
	this.insertContractVersion(ctx, tx, next)

	for name, schemaChange := range diff.Schemas {
		switch schemaChange.Kind {
		case model.ChangeAdded:
			this.insertAddedSchema(ctx, tx, next, name, next.Schemas[name])
		case model.ChangeModified:
			this.insertFieldChanges(ctx, tx, next, name, schemaChange.Fields)
		case model.ChangeRemoved:
			this.insertRemovedSchema(ctx, tx, next, name, schemaChange.Fields)
		}
	}

	for hash, kind := range diff.Resources {
		switch kind {
		case model.ChangeAdded:
			resource := next.Resources[hash]
			resourceID := this.insertResource(ctx, tx, next.ParticipantID, &resource)
			this.insertResourceVersion(ctx, tx, resourceID, next.ID, model.ChangeAdded)
		case model.ChangeRemoved:
			resourceID, _ := this.findResourceIDByHash(ctx, tx, hash, current.Resources[hash].Direction)
			this.insertResourceVersion(ctx, tx, resourceID, next.ID, model.ChangeRemoved)
		}
	}

	this.insertResourceBodies(ctx, tx, next)
}

// aliasVersionToSnapshot points a new version at an existing snapshot with the same
// content, reporting whether one matched. The alias still stamps its own contract content:
// the files are a property of the version, not of the snapshot they resolve to.
func (this *ContractRepository) aliasVersionToSnapshot(
	ctx context.Context,
	tx pgx.Tx,
	contract *model.UploadedContract,
) bool {
	tag, err := tx.Exec(
		ctx,
		aliasVersionToSnapshotQuery,
		contract.ParticipantID,
		contract.Version,
		contract.Checksum(),
		toContractContent(contract.Files),
	)
	if err != nil {
		panic(fmt.Errorf("error aliasing version to snapshot: %w", err))
	}

	return tag.RowsAffected() > 0
}

func (this *ContractRepository) insertContractVersion(
	ctx context.Context,
	tx pgx.Tx,
	contract *model.UploadedContract,
) {
	if _, err := tx.Exec(
		ctx,
		insertContractVersionQuery,
		contract.ParticipantID,
		contract.Version,
		contract.ID,
		toContractContent(contract.Files),
	); err != nil {
		panic(fmt.Errorf("error inserting contract version: %w", err))
	}
}

func (this *ContractRepository) insertContract(
	ctx context.Context,
	tx pgx.Tx,
	contract *model.UploadedContract,
) {
	if err := tx.QueryRow(
		ctx,
		insertContractQuery,
		contract.ParticipantID,
		contract.Checksum(),
	).Scan(&contract.ID); err != nil {
		panic(fmt.Errorf("error inserting contract: %w", err))
	}
}

func (this *ContractRepository) insertResource(
	ctx context.Context,
	tx pgx.Tx,
	participantID int64,
	resource *model.UploadedResource,
) int64 {
	if id, ok := this.findResourceIDByHash(ctx, tx, resource.PrimaryHash(), resource.Direction); ok {
		return id
	}

	var id int64
	if err := tx.QueryRow(
		ctx,
		insertResourceQuery,
		participantID,
		resource.Direction.String(),
		resource.Protocol.String(),
		resource.Body.String(),
		resource.ConsumedProvider,
		resource.ProviderHash(),
		null.NewString(resource.ConsumerHash(), resource.IsConsumer()),
	).Scan(&id); err != nil {
		panic(fmt.Errorf("error inserting resource: %w", err))
	}

	this.tableOf(id, resource.Protocol).Insert(ctx, tx, id, resource.Identity)

	return id
}

func (this *ContractRepository) findResourceIDByHash(
	ctx context.Context,
	tx pgx.Tx,
	hash string,
	direction model.Direction,
) (int64, bool) {
	if direction == model.Provides {
		return findID(ctx, tx, findResourceIDByProviderHashQuery, hash)
	}

	return findID(ctx, tx, findResourceIDByConsumerHashQuery, hash)
}

func (this *ContractRepository) insertResourceVersion(
	ctx context.Context,
	tx pgx.Tx,
	resourceID int64,
	contractID int64,
	kind model.ChangeKind,
) {
	if _, err := tx.Exec(ctx, insertResourceVersionQuery, resourceID, contractID, string(kind)); err != nil {
		panic(fmt.Errorf("error inserting resource version: %w", err))
	}
}

// insertResourceBodies writes the bodies of every live resource of the snapshot, after its schemas.
func (this *ContractRepository) insertResourceBodies(
	ctx context.Context,
	tx pgx.Tx,
	contract *model.UploadedContract,
) {
	schemaIDs := this.findSchemaIDsByName(ctx, tx, contract.ParticipantID)

	for hash, resource := range contract.Resources {
		resourceID, _ := this.findResourceIDByHash(ctx, tx, hash, resource.Direction)

		for _, body := range resource.Bodies {
			schemaID := null.NewInt(schemaIDs[body.SchemaName.String], body.SchemaName.Valid)

			if _, err := tx.Exec(ctx, insertResourceBodyQuery, resourceID, contract.ID, body.Source, schemaID); err != nil {
				panic(fmt.Errorf("error inserting resource body: %w", err))
			}
		}
	}
}

func (this *ContractRepository) GetLatestSnapshotByParticipantID(
	ctx context.Context,
	participantID int64,
) (*model.PersistedContract, bool) {
	return this.latestSnapshot(ctx, this.pool, participantID)
}

func (this *ContractRepository) latestSnapshot(
	ctx context.Context,
	q Querier,
	participantID int64,
) (*model.PersistedContract, bool) {
	rows, err := q.Query(ctx, getLatestSnapshotByParticipantID, participantID)
	if err != nil {
		panic(fmt.Errorf("error loading contract tree: %w", err))
	}

	return this.withSchemas(ctx, q, this.toPersistedContractTree(ctx, q, rows))
}

func (this *ContractRepository) GetContractByNameAndVersion(
	ctx context.Context,
	participantName string,
	version string,
) (*model.PersistedContract, bool) {
	rows, err := this.pool.Query(ctx, getContractByNameAndVersion, participantName, version)
	if err != nil {
		panic(fmt.Errorf("error loading contract tree by name and version: %w", err))
	}

	return this.withSchemas(ctx, this.pool, this.toPersistedContractTree(ctx, this.pool, rows))
}

func (this *ContractRepository) GetContractByIDAndVersion(
	ctx context.Context,
	contractID int64,
	version string,
) *model.PersistedContract {
	rows, err := this.pool.Query(ctx, getContractByIDAndVersion, contractID, version)
	if err != nil {
		panic(fmt.Errorf("error loading contract tree by id and version: %w", err))
	}

	contract, found := this.withSchemas(ctx, this.pool, this.toPersistedContractTree(ctx, this.pool, rows))
	if !found {
		panic(fmt.Errorf("contract %d has no version %q", contractID, version))
	}

	return contract
}

func (this *ContractRepository) withSchemas(
	ctx context.Context,
	q Querier,
	contract *model.PersistedContract,
) (*model.PersistedContract, bool) {
	if contract == nil {
		return nil, false
	}

	contract.Schemas = this.loadSchemasAt(ctx, q, schemaCatalogs{}, contract.ID)

	for hash, resource := range contract.Resources {
		resource.Schemas = contract.Schemas
		contract.Resources[hash] = resource
	}

	return contract, true
}

func (this *ContractRepository) toPersistedContractTree(ctx context.Context, q Querier, rows pgx.Rows) *model.PersistedContract {
	var contract *model.PersistedContract

	tableRows := scanTableRows(rows)
	identities := this.identitiesOf(ctx, q, tableRows)

	for _, row := range tableRows {
		if contract == nil {
			contract = row.toPersistedContractModel()
		}

		resource := row.toResourceModel(identities[row.ResourceID])
		contract.Resources[resource.PrimaryHash()] = resource
	}

	return contract
}
