package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrProviderResourceNotFound = errors.New("provider resource not found")
)

const (
	hasContractForVersionQuery = `
		SELECT EXISTS(SELECT 1 FROM contract_versions WHERE participant_id = $1 AND version = $2)
	`

	loadChecksumForVersionQuery = `
		SELECT c.checksum
		FROM contract_versions cv
		JOIN contracts c ON c.id = cv.contract_id
		WHERE cv.participant_id = $1 AND cv.version = $2
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
				interaction,
				consumed_provider,
				endpoint,
				method,
				response_status_code,
				provider_hash,
				consumer_hash
			)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9)
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

	getLatestContractByName = `
		SELECT
			c.id,
			(SELECT version FROM contract_versions WHERE contract_id = c.id ORDER BY id DESC LIMIT 1),
			(SELECT contract_content FROM contract_versions WHERE contract_id = c.id ORDER BY id DESC LIMIT 1),
			c.created_at,
			pa.id,
			pa.name,
			r.id,
			r.direction,
			r.interaction,
			r.consumed_provider,
			r.endpoint,
			r.method,
			r.response_status_code,
			r.provider_hash,
			r.consumer_hash,
			r.created_at,
			rv.change_type,
			bodies.sources,
			bodies.schema_names
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
		WHERE pa.name = $1
		  AND c.id = (SELECT MAX(id) FROM contracts WHERE participant_id = pa.id)
		  AND rv.change_type = 'added'
		ORDER BY r.id
	`

	getContractByNameAndVersion = `
		SELECT
			c.id,
			cv.version,
			cv.contract_content,
			c.created_at,
			pa.id,
			pa.name,
			r.id,
			r.direction,
			r.interaction,
			r.consumed_provider,
			r.endpoint,
			r.method,
			r.response_status_code,
			r.provider_hash,
			r.consumer_hash,
			r.created_at,
			rv.change_type,
			bodies.sources,
			bodies.schema_names
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
		WHERE pa.name = $1
		  AND cv.version = $2
		ORDER BY r.id
	`

	consumersByProviderHashAndEnvironmentQuery = `
		SELECT
			pa.id,
			pa.name,
			r.id,
			r.direction,
			r.interaction,
			r.consumed_provider,
			r.endpoint,
			r.method,
			r.response_status_code,
			r.provider_hash,
			r.consumer_hash,
			r.created_at,
			bodies.sources,
			bodies.schema_names,
			dep.version,
			anchor.contract_id
		FROM
			resources r
		JOIN
			participants pa ON pa.id = r.participant_id
		JOIN LATERAL (
			SELECT version
			FROM deployments
			WHERE participant_id = r.participant_id
			  AND environment_id = $3
			ORDER BY deployed_at DESC
			LIMIT 1
		) dep ON true
		JOIN LATERAL (
			SELECT COALESCE(
				(SELECT cv.contract_id FROM contract_versions cv
				  WHERE cv.participant_id = r.participant_id
				    AND cv.version = dep.version),
				(SELECT MAX(id) FROM contracts WHERE participant_id = r.participant_id)
			) AS contract_id
		) anchor ON true
		JOIN LATERAL (
			SELECT change_type
			FROM resource_versions
			WHERE resource_id = r.id
			  AND contract_id <= anchor.contract_id
			ORDER BY contract_id DESC
			LIMIT 1
		) rv ON true
		JOIN LATERAL (
			SELECT
				array_agg(rb.source ORDER BY rb.source COLLATE "C", rb.id) AS sources,
				array_agg(s.name ORDER BY rb.source COLLATE "C", rb.id) AS schema_names
			FROM resource_bodies rb
			LEFT JOIN schemas s ON s.id = rb.schema_id
			WHERE rb.resource_id = r.id AND rb.contract_id = anchor.contract_id
		) bodies ON true
		WHERE
			r.direction = $1
		AND
			r.provider_hash = $2
		AND
			rv.change_type = 'added'
	`

	loadProviderResourceWithDeploymentsQuery = `
		SELECT
			pa.id,
			pa.name,
			r.id,
			r.direction,
			r.interaction,
			r.consumed_provider,
			r.endpoint,
			r.method,
			r.response_status_code,
			r.provider_hash,
			r.consumer_hash,
			r.created_at,
			bodies.sources,
			bodies.schema_names,
			e.name,
			dep.version,
			anchor.contract_id
		FROM
			resources r
		JOIN
			participants pa ON pa.id = r.participant_id
		JOIN LATERAL (
			SELECT COALESCE(
				(SELECT cv.contract_id FROM contract_versions cv
				  WHERE cv.participant_id = r.participant_id
				    AND cv.version = (
					SELECT version
					FROM deployments
					WHERE participant_id = r.participant_id
					  AND environment_id = $3
					ORDER BY deployed_at DESC
					LIMIT 1
				 )),
				(SELECT MAX(id) FROM contracts WHERE participant_id = r.participant_id)
			) AS contract_id
		) anchor ON true
		JOIN LATERAL (
			SELECT change_type
			FROM resource_versions
			WHERE resource_id = r.id
			  AND contract_id <= anchor.contract_id
			ORDER BY contract_id DESC
			LIMIT 1
		) rv ON true
		JOIN LATERAL (
			SELECT
				array_agg(rb.source ORDER BY rb.source COLLATE "C", rb.id) AS sources,
				array_agg(s.name ORDER BY rb.source COLLATE "C", rb.id) AS schema_names
			FROM resource_bodies rb
			LEFT JOIN schemas s ON s.id = rb.schema_id
			WHERE rb.resource_id = r.id AND rb.contract_id = anchor.contract_id
		) bodies ON true
		LEFT JOIN LATERAL (
			SELECT DISTINCT ON (environment_id) environment_id, version
			FROM deployments
			WHERE participant_id = r.participant_id
			ORDER BY environment_id, deployed_at DESC
		) dep ON true
		LEFT JOIN
			environments e ON e.id = dep.environment_id
		WHERE
			r.direction = $1
		AND
			r.provider_hash = $2
		AND
			rv.change_type = 'added'
	`
)

type ContractRepository struct {
	pool *pgxpool.Pool
}

func NewContractRepository(pool *pgxpool.Pool) *ContractRepository {
	return &ContractRepository{pool: pool}
}

func (this *ContractRepository) HasContractForVersion(ctx context.Context, participantID int64, version string) bool {
	var exists bool

	if err := this.pool.QueryRow(ctx, hasContractForVersionQuery, participantID, version).Scan(&exists); err != nil {
		panic(fmt.Errorf("error checking contract version: %w", err))
	}

	return exists
}

func (this *ContractRepository) LoadChecksumForVersion(ctx context.Context, participantID int64, version string) (string, bool) {
	var checksum string

	err := this.pool.QueryRow(ctx, loadChecksumForVersionQuery, participantID, version).Scan(&checksum)
	if err == nil {
		return checksum, true
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false
	}
	panic(fmt.Errorf("error loading checksum for version: %w", err))
}

func (this *ContractRepository) Create(
	ctx context.Context,
	contract *model.UploadedContract,
) {
	tx, err := this.pool.Begin(ctx)
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

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

	if err := tx.Commit(ctx); err != nil {
		panic(fmt.Errorf("error committing transaction: %w", err))
	}
}

func (this *ContractRepository) Update(
	ctx context.Context,
	next *model.UploadedContract,
	current *model.PersistedContract,
	diff model.ContractDiff,
) {
	tx, err := this.pool.Begin(ctx)
	if err != nil {
		panic(fmt.Errorf("error starting transaction: %w", err))
	}

	defer func() { _ = tx.Rollback(ctx) }()

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

	if err := tx.Commit(ctx); err != nil {
		panic(fmt.Errorf("error committing transaction: %w", err))
	}
}

// AliasVersionToSnapshot points a new version at an existing snapshot with the same
// content, reporting whether one matched. The alias still stamps its own contract content:
// the files are a property of the version, not of the snapshot they resolve to.
func (this *ContractRepository) AliasVersionToSnapshot(
	ctx context.Context,
	contract *model.UploadedContract,
) bool {
	tag, err := this.pool.Exec(
		ctx,
		aliasVersionToSnapshotQuery,
		contract.ParticipantID,
		contract.Version,
		contract.Checksum(),
		contract.ContractContent,
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
		contract.ContractContent,
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

	statusCode := sql.NullString{
		String: resource.ResponseStatusCode.String,
		Valid:  resource.ResponseStatusCode.Valid,
	}

	provider := sql.NullString{
		String: resource.ConsumedProvider.String,
		Valid:  resource.ConsumedProvider.Valid,
	}

	providerHash := sql.NullString{
		String: resource.ProviderHash(),
		Valid:  resource.ProviderHash() != "",
	}

	consumerHash := sql.NullString{
		String: resource.ConsumerHash(),
		Valid:  resource.ConsumerHash() != "",
	}

	var id int64
	if err := tx.QueryRow(
		ctx,
		insertResourceQuery,
		participantID,
		resource.Direction.String(),
		resource.Interaction.String(),
		provider,
		resource.Endpoint,
		resource.Method,
		statusCode,
		providerHash,
		consumerHash,
	).Scan(&id); err != nil {
		panic(fmt.Errorf("error inserting resource: %w", err))
	}

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
			schemaID := sql.NullInt64{Int64: schemaIDs[body.SchemaName.String], Valid: body.SchemaName.Valid}

			if _, err := tx.Exec(ctx, insertResourceBodyQuery, resourceID, contract.ID, body.Source, schemaID); err != nil {
				panic(fmt.Errorf("error inserting resource body: %w", err))
			}
		}
	}
}

func (this *ContractRepository) GetLatestContractByName(
	ctx context.Context,
	participantName string,
) (*model.PersistedContract, bool) {
	rows, err := this.pool.Query(ctx, getLatestContractByName, participantName)
	if err != nil {
		panic(fmt.Errorf("error loading contract tree: %w", err))
	}

	return this.withSchemas(ctx, scanPersistedContractTree(rows))
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

	return this.withSchemas(ctx, scanPersistedContractTree(rows))
}

func (this *ContractRepository) withSchemas(
	ctx context.Context,
	contract *model.PersistedContract,
) (*model.PersistedContract, bool) {
	if contract == nil {
		return nil, false
	}

	contract.Schemas = this.loadSchemasAt(ctx, schemaCatalogs{}, contract.ID)

	for hash, resource := range contract.Resources {
		resource.Schemas = contract.Schemas
		contract.Resources[hash] = resource
	}

	return contract, true
}

func scanPersistedContractTree(rows pgx.Rows) *model.PersistedContract {
	defer rows.Close()

	var contract *model.PersistedContract

	for rows.Next() {
		var row tableRow

		if err := rows.Scan(
			&row.ContractID,
			&row.ContractVersion,
			&row.ContractContent,
			&row.ContractCreatedAt,
			&row.ParticipantID,
			&row.ParticipantName,
			&row.ResourceID,
			&row.ResourceDirection,
			&row.ResourceInteraction,
			&row.ResourceConsumedProvider,
			&row.ResourceEndpoint,
			&row.ResourceMethod,
			&row.ResourceResponseStatusCode,
			&row.ResourceProviderHash,
			&row.ResourceConsumerHash,
			&row.ResourceCreatedAt,
			&row.ResourceVersionChangeType,
			&row.BodySources,
			&row.BodySchemaNames,
		); err != nil {
			panic(fmt.Errorf("error scanning contract tree row: %w", err))
		}

		if contract == nil {
			contract = row.toPersistedContractModel()
		}

		resource := row.toResourceModel()
		contract.Resources[resource.PrimaryHash()] = resource
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading contract tree: %w", err))
	}

	return contract
}

type CurrentConsumerInEnv struct {
	ParticipantID   int64
	ParticipantName string
	Version         string
}

func (this *ContractRepository) GetProviderResourceByConsumerResource(
	ctx context.Context,
	providerHash string,
	environmentID int64,
) (model.PersistedResource, error) {
	return this.loadProviderResource(ctx, schemaCatalogs{}, providerHash, environmentID)
}

func (this *ContractRepository) GetConsumersResourcesByProviderHashAndEnvironmentID(
	ctx context.Context,
	providerHash string,
	environmentID int64,
) []model.PersistedResource {
	return this.loadConsumerResources(ctx, schemaCatalogs{}, providerHash, environmentID)
}

func (this *ContractRepository) loadProviderResource(
	ctx context.Context,
	catalogs schemaCatalogs,
	providerHash string,
	environmentID int64,
) (model.PersistedResource, error) {
	rows, err := this.pool.Query(
		ctx,
		loadProviderResourceWithDeploymentsQuery,
		string(model.Provides),
		providerHash,
		environmentID,
	)
	if err != nil {
		panic(fmt.Errorf("error loading provider resource of consumer: %w", err))
	}

	defer rows.Close()

	var found bool
	var provider model.PersistedResource

	for rows.Next() {
		var row tableRow

		if err := rows.Scan(
			&row.ParticipantID,
			&row.ParticipantName,
			&row.ResourceID,
			&row.ResourceDirection,
			&row.ResourceInteraction,
			&row.ResourceConsumedProvider,
			&row.ResourceEndpoint,
			&row.ResourceMethod,
			&row.ResourceResponseStatusCode,
			&row.ResourceProviderHash,
			&row.ResourceConsumerHash,
			&row.ResourceCreatedAt,
			&row.BodySources,
			&row.BodySchemaNames,
			&row.DeploymentEnvironment,
			&row.DeploymentVersion,
			&row.ContractID,
		); err != nil {
			panic(fmt.Errorf("error scanning provider resource of consumer: %w", err))
		}

		if !found {
			provider = row.toResourceModel()
			found = true
		}

		if row.DeploymentEnvironment.Valid {
			provider.DeployedVersions[row.DeploymentEnvironment.String] = row.DeploymentVersion.String
		}
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading provider resource of consumer: %w", err))
	}

	rows.Close()

	if !found {
		return model.PersistedResource{}, ErrProviderResourceNotFound
	}

	provider.Schemas = this.loadSchemasAt(ctx, catalogs, provider.ContractID)

	return provider, nil
}

func (this *ContractRepository) loadConsumerResources(
	ctx context.Context,
	catalogs schemaCatalogs,
	providerHash string,
	environmentID int64,
) []model.PersistedResource {
	rows, err := this.pool.Query(
		ctx,
		consumersByProviderHashAndEnvironmentQuery,
		string(model.Consumes),
		providerHash,
		environmentID,
	)
	if err != nil {
		panic(fmt.Errorf("error finding consumers of provider: %w", err))
	}

	defer rows.Close()

	consumers := make([]model.PersistedResource, 0)

	for rows.Next() {
		var row tableRow

		if err := rows.Scan(
			&row.ParticipantID,
			&row.ParticipantName,
			&row.ResourceID,
			&row.ResourceDirection,
			&row.ResourceInteraction,
			&row.ResourceConsumedProvider,
			&row.ResourceEndpoint,
			&row.ResourceMethod,
			&row.ResourceResponseStatusCode,
			&row.ResourceProviderHash,
			&row.ResourceConsumerHash,
			&row.ResourceCreatedAt,
			&row.BodySources,
			&row.BodySchemaNames,
			&row.ResourceVersion,
			&row.ContractID,
		); err != nil {
			panic(fmt.Errorf("error scanning consumer: %w", err))
		}

		consumers = append(consumers, row.toResourceModel())
	}

	if err := rows.Err(); err != nil {
		panic(fmt.Errorf("error reading consumers of provider: %w", err))
	}

	rows.Close()

	for index := range consumers {
		consumers[index].Schemas = this.loadSchemasAt(ctx, catalogs, consumers[index].ContractID)
	}

	return consumers
}
