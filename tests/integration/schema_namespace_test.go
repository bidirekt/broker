package integration_test

import (
	"context"
	"io/fs"
	"net/http"
	"testing/fstest"

	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
	"github.com/bidirekt/broker/internal/repository"
	restrepository "github.com/bidirekt/broker/internal/repository/protocols/rest"
	"github.com/bidirekt/broker/migrations"
	"github.com/bidirekt/broker/pkg/migrator"
	"github.com/guregu/null"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	schemaNamespaceDatabase        = "schema_namespace"
	schemaNamespaceMigrationsTable = "public.schema_migrations"
	schemaNamespaceMigration       = "20261124130000_schema_namespace.sql"
)

const schemaNamespaceProviderYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
schemas:
  Pet:
    type: object
    properties:
      id: { type: string }
      name: { type: string }
`

const schemaNamespaceConsumerYAML = `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: Pet
schemas:
  Pet:
    type: object
    properties:
      id: { type: string }
      name: { type: string }
`

const schemaNamespaceOrdersYAML = `provides:
  rest:
    /orders:
      post:
        request: Order
        responses:
          201: ~
schemas:
  Order:
    type: object
    properties:
      billing:
        ref: Address
  Address:
    type: object
    properties:
      city:
        type: string
`

const schemaNamespaceOrdersChecksum = "e740afc52d49a0842198eaa90fa8690cd18369b9fbaf8215aeee6e606b5abff3"

func (this *IntegrationSuite) TestSchemaNamespaceMigration_KeepsTheSchemasAlreadyStoredInTheYAMLNamespace() {
	ctx := context.Background()

	pool := this.openSchemaNamespaceDatabase(ctx)
	this.Require().NoError(migrator.New(pool, this.migrationsBeforeSchemaNamespace(), schemaNamespaceMigrationsTable).Migrate())
	_, err := pool.Exec(ctx, `
		INSERT INTO participants (id, name) VALUES (1, 'petstore_api');
		INSERT INTO schemas (participant_id, name) VALUES (1, 'Pet'), (1, 'NewPet');
	`)
	this.Require().NoError(err)

	this.Require().NoError(migrator.New(pool, migrations.FS, schemaNamespaceMigrationsTable).Migrate())

	var namespaces []string
	rows, err := pool.Query(ctx, `SELECT namespace || ' ' || name FROM schemas ORDER BY name`)
	this.Require().NoError(err)
	for rows.Next() {
		var namespace string
		this.Require().NoError(rows.Scan(&namespace))
		namespaces = append(namespaces, namespace)
	}
	this.Require().NoError(rows.Err())
	this.Equal([]string{"yaml NewPet", "yaml Pet"}, namespaces)

	_, err = pool.Exec(ctx, `INSERT INTO schemas (participant_id, namespace, name) VALUES (1, 'graphql', 'Pet')`)
	this.NoError(err, "the same name in another namespace")
	_, err = pool.Exec(ctx, `INSERT INTO schemas (participant_id, namespace, name) VALUES (1, 'yaml', 'Pet')`)
	this.ErrorContains(err, "schemas_participant_id_namespace_name_key", "the same name in the same namespace")
}

func (this *IntegrationSuite) TestSchemaNamespace_SameNameInTwoNamespaces_CoexistAndARestResourceLoadsOnlyTheYAMLCatalog() {
	ctx := context.Background()
	repo := repository.NewContractRepository(this.Pool, []repository.ProtocolTable{restrepository.New()})

	var participantID int64
	this.Require().NoError(this.Pool.QueryRow(ctx, `INSERT INTO participants (name) VALUES ('petstore_api') RETURNING id`).Scan(&participantID))

	yamlPet := model.Schema{"": {Type: "object"}, "id": {Type: "string"}}
	graphqlPet := model.Schema{"": {Type: "object"}, "id": {Type: "integer"}}
	contract := model.NewUploadedContract(participantID, "petstore_api", "1", []model.ContractFile{{Source: "api.yaml", Content: "provides: {}"}})
	contract.Schemas = map[string]model.Schemas{
		model.YAMLNamespace: {"Pet": yamlPet},
		"graphql":           {"Pet": graphqlPet},
	}
	this.Require().NoError(contract.AddResource(&model.UploadedResource{Resource: model.Resource{
		Direction: model.Provides,
		Protocol:  restmodel.Protocol,
		Body:      model.Output,
		Identity:  restmodel.Identity{Endpoint: "/pets", Method: "get", Status: "200"},
		Bodies:    []model.ResourceBody{{Source: "api.yaml", Namespace: model.YAMLNamespace, SchemaName: null.StringFrom("Pet")}},
	}}))

	conflict := repo.Publish(ctx, contract,
		func(*model.PersistedContract) model.ContractDiff { return model.ContractDiff{} },
		func([]model.ContractFile) bool { return true })
	this.Require().False(conflict)

	var bodyNamespace string
	this.Require().NoError(this.Pool.QueryRow(ctx, `
		SELECT s.namespace FROM resource_bodies rb JOIN schemas s ON s.id = rb.schema_id`).Scan(&bodyNamespace))
	this.Equal(model.YAMLNamespace, bodyNamespace)

	persisted, found := repo.GetContractByNameAndVersion(ctx, "petstore_api", "1")
	this.Require().True(found)
	this.Equal(map[string]model.Schemas{
		model.YAMLNamespace: {"Pet": yamlPet},
		"graphql":           {"Pet": graphqlPet},
	}, persisted.Schemas)
	this.Require().Len(persisted.Resources, 1)
	for _, resource := range persisted.Resources {
		this.Equal(map[string]model.Schemas{model.YAMLNamespace: {"Pet": yamlPet}}, resource.Schemas)
	}
}

func (this *IntegrationSuite) TestSchemaNamespace_CanIDeploy_ComparesARestConsumerWithTheYAMLSchema() {
	ctx := context.Background()
	this.mustPostForSchemaNamespace("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForSchemaNamespace("/api/contracts", this.publishBody("petstore_api", "1.0.0",
		contractFragment{"api.yaml", schemaNamespaceProviderYAML}))

	_, err := this.Pool.Exec(ctx, `
		WITH provider AS (
			SELECT c.id AS contract_id, c.participant_id
			FROM contracts c JOIN participants p ON p.id = c.participant_id
			WHERE p.name = 'petstore_api'
		), pet AS (
			INSERT INTO schemas (participant_id, namespace, name)
			SELECT participant_id, 'graphql', 'Pet' FROM provider
			RETURNING id
		), pet_version AS (
			INSERT INTO schema_versions (schema_id, contract_id, change_type)
			SELECT pet.id, provider.contract_id, 'added' FROM pet, provider
		), pet_fields AS (
			INSERT INTO fields (schema_id, path)
			SELECT pet.id, path FROM pet, (VALUES (''), ('id')) AS paths(path)
			RETURNING id, path
		)
		INSERT INTO field_versions (field_id, contract_id, type, optional, nullable, change_type)
		SELECT pet_fields.id, provider.contract_id, CASE WHEN path = '' THEN 'object' ELSE 'integer' END, false, false, 'added'
		FROM pet_fields, provider`)
	this.Require().NoError(err)

	this.mustPostForSchemaNamespace("/api/environments", `{"environment":"production"}`)
	this.mustPostForSchemaNamespace("/api/deployments", `{"participant":"petstore_api","version":"1.0.0","environment":"production"}`)
	this.mustPostForSchemaNamespace("/api/participants", `{"participant":"petstore_web"}`)
	this.mustPostForSchemaNamespace("/api/contracts", this.publishBody("petstore_web", "1.0.0",
		contractFragment{"web.yaml", schemaNamespaceConsumerYAML}))

	this.Equal(2, this.countSchemasNamedPetForSchemaNamespace(ctx, "petstore_api"))

	status, body := this.post("/api/can-i-deploy", `{"participant":"petstore_web","version":"1.0.0","environment":"production"}`)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract checked successfully","participant":"petstore_web","version":"1.0.0","environment":"production","deployable":true,"results":{"petstore_api":{"deployable":true,"participantVersion":"1.0.0"}}}`, body)
}

func (this *IntegrationSuite) TestSchemaNamespace_RestOnlyContract_KeepsItsChecksum() {
	this.mustPostForSchemaNamespace("/api/participants", `{"participant":"orders_api"}`)
	this.mustPostForSchemaNamespace("/api/contracts", this.publishBody("orders_api", "1",
		contractFragment{"api.yaml", schemaNamespaceOrdersYAML}))

	var checksum string
	this.Require().NoError(this.Pool.QueryRow(context.Background(), `SELECT checksum FROM contracts`).Scan(&checksum))
	this.Equal(schemaNamespaceOrdersChecksum, checksum)
}

func (this *IntegrationSuite) mustPostForSchemaNamespace(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) countSchemasNamedPetForSchemaNamespace(ctx context.Context, participant string) int {
	var count int
	this.Require().NoError(this.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM schemas s JOIN participants p ON p.id = s.participant_id
		WHERE p.name = $1 AND s.name = 'Pet'`, participant).Scan(&count))

	return count
}

func (this *IntegrationSuite) migrationsBeforeSchemaNamespace() fs.FS {
	entries, err := fs.ReadDir(migrations.FS, ".")
	this.Require().NoError(err)

	earlier := fstest.MapFS{}
	for _, entry := range entries {
		if entry.Name() >= schemaNamespaceMigration {
			continue
		}
		contents, err := fs.ReadFile(migrations.FS, entry.Name())
		this.Require().NoError(err)
		earlier[entry.Name()] = &fstest.MapFile{Data: contents}
	}

	return earlier
}

func (this *IntegrationSuite) openSchemaNamespaceDatabase(ctx context.Context) *pgxpool.Pool {
	_, err := this.Pool.Exec(ctx, `DROP DATABASE IF EXISTS `+schemaNamespaceDatabase+` WITH (FORCE)`)
	this.Require().NoError(err)

	_, err = this.Pool.Exec(ctx, `CREATE DATABASE `+schemaNamespaceDatabase)
	this.Require().NoError(err)

	config := this.Pool.Config()
	config.ConnConfig.Database = schemaNamespaceDatabase

	pool, err := pgxpool.NewWithConfig(ctx, config)
	this.Require().NoError(err)

	suitePool := this.Pool
	this.T().Cleanup(func() {
		pool.Close()
		_, err := suitePool.Exec(ctx, `DROP DATABASE `+schemaNamespaceDatabase+` WITH (FORCE)`)
		this.Require().NoError(err)
	})

	return pool
}
