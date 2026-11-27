package integration_test

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"testing/fstest"

	"github.com/bidirekt/broker/internal"
	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
	"github.com/bidirekt/broker/migrations"
	"github.com/bidirekt/broker/pkg/migrator"
	"github.com/guregu/null"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	hashProtocolDatabase        = "resource_hash_protocol"
	hashProtocolMigrationsTable = "public.schema_migrations"
	hashProtocolMigration       = "20261124120000_resource_hash_protocol.sql"
)

const hashProtocolProviderYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
      post:
        request: NewPet
schemas:
  Pet:
    type: object
    properties:
      id: { type: string }
      name: { type: string }
  NewPet:
    type: object
    properties:
      name: { type: string }
`

const hashProtocolProviderWithoutIDYAML = `provides:
  rest:
    /pets:
      get:
        responses:
          200: Pet
      post:
        request: NewPet
schemas:
  Pet:
    type: object
    properties:
      name: { type: string }
  NewPet:
    type: object
    properties:
      name: { type: string }
`

const hashProtocolConsumerYAML = `consumes:
  petstore_api:
    rest:
      /pets:
        get:
          responses:
            200: Pet
        post:
          request: NewPet
schemas:
  Pet:
    type: object
    properties:
      id: { type: string }
  NewPet:
    type: object
    properties:
      name: { type: string }
`

// Written by the broker before the protocol entered the resource hash: petstore_api 1.0.0 and
// petstore_web 1.0.0 deployed in production, their hashes Hash(provider, endpoint, method, status).
const hashProtocolOldPetstoreData = `
INSERT INTO participants (id, name) VALUES
  (1, 'petstore_api'),
  (2, 'petstore_web');
INSERT INTO contracts (id, participant_id, checksum) VALUES
  (1, 1, 'old-petstore-api-checksum'),
  (2, 2, 'old-petstore-web-checksum');
INSERT INTO environments (id, name) VALUES
  (1, 'production');
INSERT INTO contract_versions (id, participant_id, version, contract_id, contract_content) VALUES
  (1, 1, '1.0.0', 1, '[{"source":"api.yaml","content":"provides:\n  rest:\n    /pets:\n      get:\n        responses:\n          200: Pet\n      post:\n        request: NewPet\nschemas:\n  Pet:\n    type: object\n    properties:\n      id: { type: string }\n      name: { type: string }\n  NewPet:\n    type: object\n    properties:\n      name: { type: string }\n"}]'),
  (2, 2, '1.0.0', 2, '[{"source":"web.yaml","content":"consumes:\n  petstore_api:\n    rest:\n      /pets:\n        get:\n          responses:\n            200: Pet\n        post:\n          request: NewPet\nschemas:\n  Pet:\n    type: object\n    properties:\n      id: { type: string }\n  NewPet:\n    type: object\n    properties:\n      name: { type: string }\n"}]');
INSERT INTO deployments (id, participant_id, version, environment_id, rollback, deployed_at) VALUES
  (1, 1, '1.0.0', 1, false, '2026-11-20 10:00:00+00'),
  (2, 2, '1.0.0', 1, false, '2026-11-20 10:05:00+00');
INSERT INTO current_deployments (participant_id, environment_id, version, contract_id, deployed_at) VALUES
  (1, 1, '1.0.0', 1, '2026-11-20 10:00:00+00'),
  (2, 1, '1.0.0', 2, '2026-11-20 10:05:00+00');
INSERT INTO schemas (id, participant_id, name) VALUES
  (1, 1, 'Pet'),
  (2, 1, 'NewPet'),
  (3, 2, 'Pet'),
  (4, 2, 'NewPet');
INSERT INTO schema_versions (id, schema_id, contract_id, change_type) VALUES
  (1, 1, 1, 'added'),
  (2, 2, 1, 'added'),
  (3, 3, 2, 'added'),
  (4, 4, 2, 'added');
INSERT INTO fields (id, schema_id, path) VALUES
  (1, 1, ''),
  (2, 1, 'id'),
  (3, 1, 'name'),
  (4, 2, ''),
  (5, 2, 'name'),
  (6, 3, ''),
  (7, 3, 'id'),
  (8, 4, ''),
  (9, 4, 'name');
INSERT INTO field_versions (id, field_id, contract_id, type, optional, nullable, ref, change_type) VALUES
  (1, 1, 1, 'object', false, false, NULL, 'added'),
  (2, 2, 1, 'string', false, false, NULL, 'added'),
  (3, 3, 1, 'string', false, false, NULL, 'added'),
  (4, 4, 1, 'object', false, false, NULL, 'added'),
  (5, 5, 1, 'string', false, false, NULL, 'added'),
  (6, 6, 2, 'object', false, false, NULL, 'added'),
  (7, 7, 2, 'string', false, false, NULL, 'added'),
  (8, 8, 2, 'object', false, false, NULL, 'added'),
  (9, 9, 2, 'string', false, false, NULL, 'added');
INSERT INTO resources (id, participant_id, direction, consumed_provider, provider_hash, consumer_hash, protocol, body_role) VALUES
  (1, 1, 'provides', NULL, '4186fd903c086826655928f1aac69b6a09dd339d11d370c19df275c56878aca4', NULL, 'rest', 'output'),
  (2, 1, 'provides', NULL, '821227ce1e0880384ff4d8d8891cafb957ebeadfde79a4df94ab3874491beedf', NULL, 'rest', 'input'),
  (3, 2, 'consumes', 'petstore_api', '4186fd903c086826655928f1aac69b6a09dd339d11d370c19df275c56878aca4', '0c6429446ddbdc49fcbc14dd1f160a345f7d30ce6bbd63660f019df6517b2243', 'rest', 'output'),
  (4, 2, 'consumes', 'petstore_api', '821227ce1e0880384ff4d8d8891cafb957ebeadfde79a4df94ab3874491beedf', '66c18366e53a4ae1638906821bab8b81376821afe29c3f1ed448dfca299e427a', 'rest', 'input');
INSERT INTO rest_resources (resource_id, protocol, endpoint, method, response_status_code) VALUES
  (1, 'rest', '/pets', 'get', '200'),
  (2, 'rest', '/pets', 'post', NULL),
  (3, 'rest', '/pets', 'get', '200'),
  (4, 'rest', '/pets', 'post', NULL);
INSERT INTO resource_versions (id, resource_id, contract_id, change_type) VALUES
  (1, 1, 1, 'added'),
  (2, 2, 1, 'added'),
  (3, 3, 2, 'added'),
  (4, 4, 2, 'added');
INSERT INTO resource_bodies (id, resource_id, contract_id, source, schema_id) VALUES
  (1, 1, 1, 'api.yaml', 1),
  (2, 2, 1, 'api.yaml', 2),
  (3, 3, 2, 'web.yaml', 3),
  (4, 4, 2, 'web.yaml', 4);
SELECT pg_catalog.setval('participants_id_seq', 2, true);
SELECT pg_catalog.setval('contracts_id_seq', 2, true);
SELECT pg_catalog.setval('environments_id_seq', 1, true);
SELECT pg_catalog.setval('contract_versions_id_seq', 2, true);
SELECT pg_catalog.setval('deployments_id_seq', 2, true);
SELECT pg_catalog.setval('schemas_id_seq', 4, true);
SELECT pg_catalog.setval('schema_versions_id_seq', 4, true);
SELECT pg_catalog.setval('fields_id_seq', 9, true);
SELECT pg_catalog.setval('field_versions_id_seq', 9, true);
SELECT pg_catalog.setval('resources_id_seq', 4, true);
SELECT pg_catalog.setval('resource_versions_id_seq', 4, true);
SELECT pg_catalog.setval('resource_bodies_id_seq', 4, true);
`

type hashProtocolStoredResource struct {
	participant      string
	direction        model.Direction
	consumedProvider null.String
	endpoint         string
	method           string
	status           null.String
	providerHash     string
	consumerHash     null.String
}

func (this *IntegrationSuite) TestResourceHashProtocolMigration_RecomputesTheHashesTheBrokerComputes() {
	ctx := context.Background()
	this.mustPostForHashProtocol("/api/participants", `{"participant":"petstore_api"}`)
	this.mustPostForHashProtocol("/api/participants", `{"participant":"petstore_web"}`)
	this.mustPostForHashProtocol("/api/contracts", this.publishBody("petstore_api", "1.0.0",
		contractFragment{"api.yaml", hashProtocolProviderYAML}))
	this.mustPostForHashProtocol("/api/contracts", this.publishBody("petstore_web", "1.0.0",
		contractFragment{"web.yaml", hashProtocolConsumerYAML}))

	_, err := this.Pool.Exec(ctx, `UPDATE resources
		SET provider_hash = 'stale-' || id,
		    consumer_hash = CASE WHEN direction = 'consumes' THEN 'stale-' || id END`)
	this.Require().NoError(err)

	migration, err := fs.ReadFile(migrations.FS, hashProtocolMigration)
	this.Require().NoError(err)
	_, err = this.Pool.Exec(ctx, string(migration))
	this.Require().NoError(err)

	stored := this.storedResourcesForHashProtocol(ctx)
	this.Require().Len(stored, 4)
	for _, row := range stored {
		resource := model.UploadedResource{Resource: model.Resource{
			ParticipantName:  row.participant,
			Direction:        row.direction,
			Protocol:         restmodel.Protocol,
			Identity:         restmodel.Identity{Endpoint: row.endpoint, Method: row.method, Status: row.status.String},
			ConsumedProvider: row.consumedProvider,
		}}

		this.Equal(resource.ProviderHash(), row.providerHash, resource.Describe())
		this.Equal(null.NewString(resource.ConsumerHash(), resource.IsConsumer()), row.consumerHash, resource.Describe())
	}
}

func (this *IntegrationSuite) TestResourceHashProtocolMigration_MatchesProviderAndConsumerWrittenWithTheOldHashes() {
	ctx := context.Background()

	pool := this.openHashProtocolDatabase(ctx)
	this.Require().NoError(migrator.New(pool, this.migrationsBeforeHashProtocol(), hashProtocolMigrationsTable).Migrate())
	_, err := pool.Exec(ctx, hashProtocolOldPetstoreData)
	this.Require().NoError(err)

	this.serveHashProtocolDatabase(pool)

	var hashes []string
	rows, err := this.Pool.Query(ctx, `SELECT provider_hash || ' ' || COALESCE(consumer_hash, '-') FROM resources ORDER BY id`)
	this.Require().NoError(err)
	for rows.Next() {
		var hash string
		this.Require().NoError(rows.Scan(&hash))
		hashes = append(hashes, hash)
	}
	this.Require().NoError(rows.Err())
	this.Equal([]string{
		model.Hash("petstore_api", "rest", "/pets", "get", "200") + " -",
		model.Hash("petstore_api", "rest", "/pets", "post") + " -",
		model.Hash("petstore_api", "rest", "/pets", "get", "200") + " " + model.Hash("petstore_web", "petstore_api", "rest", "/pets", "get", "200"),
		model.Hash("petstore_api", "rest", "/pets", "post") + " " + model.Hash("petstore_web", "petstore_api", "rest", "/pets", "post"),
	}, hashes)

	status, body := this.post("/api/contracts", this.publishBody("petstore_api", "1.0.0",
		contractFragment{"api.yaml", hashProtocolProviderYAML}))
	this.Equal(http.StatusOK, status, body)

	this.mustPostForHashProtocol("/api/contracts", this.publishBody("petstore_web", "2.0.0",
		contractFragment{"web.yaml", hashProtocolConsumerYAML}))
	status, body = this.post("/api/can-i-deploy", `{"participant":"petstore_web","version":"2.0.0","environment":"production"}`)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract checked successfully","participant":"petstore_web","version":"2.0.0","environment":"production","deployable":true,"results":{"petstore_api":{"deployable":true,"participantVersion":"1.0.0","endpoints":{}}}}`, body)

	this.mustPostForHashProtocol("/api/contracts", this.publishBody("petstore_api", "2.0.0",
		contractFragment{"api.yaml", hashProtocolProviderWithoutIDYAML}))
	status, body = this.post("/api/can-i-deploy", `{"participant":"petstore_api","version":"2.0.0","environment":"production"}`)
	this.Equal(http.StatusOK, status)
	this.JSONEq(`{"message":"contract checked successfully","participant":"petstore_api","version":"2.0.0","environment":"production","deployable":false,"results":{"petstore_web":{"deployable":false,"participantVersion":"1.0.0","endpoints":{"/pets":{"get":{"200":[{"reason":"property_missing_in_provider","role":"provider","details":{"consumerField":"Pet.id","consumerName":"petstore_web","property":"$.id","propertyType":"string","providerName":"petstore_api"}}]}}}}}}`, body)
}

func (this *IntegrationSuite) mustPostForHashProtocol(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) storedResourcesForHashProtocol(ctx context.Context) []hashProtocolStoredResource {
	rows, err := this.Pool.Query(ctx, `
		SELECT p.name, r.direction, r.consumed_provider, rr.endpoint, rr.method, rr.response_status_code, r.provider_hash, r.consumer_hash
		FROM resources r
		JOIN rest_resources rr ON rr.resource_id = r.id
		JOIN participants p ON p.id = r.participant_id
		ORDER BY r.id`)
	this.Require().NoError(err)
	defer rows.Close()

	var stored []hashProtocolStoredResource
	for rows.Next() {
		var row hashProtocolStoredResource
		this.Require().NoError(rows.Scan(&row.participant, &row.direction, &row.consumedProvider, &row.endpoint, &row.method, &row.status, &row.providerHash, &row.consumerHash))
		stored = append(stored, row)
	}
	this.Require().NoError(rows.Err())

	return stored
}

func (this *IntegrationSuite) migrationsBeforeHashProtocol() fs.FS {
	entries, err := fs.ReadDir(migrations.FS, ".")
	this.Require().NoError(err)

	earlier := fstest.MapFS{}
	for _, entry := range entries {
		if entry.Name() >= hashProtocolMigration {
			continue
		}
		contents, err := fs.ReadFile(migrations.FS, entry.Name())
		this.Require().NoError(err)
		earlier[entry.Name()] = &fstest.MapFile{Data: contents}
	}

	return earlier
}

func (this *IntegrationSuite) openHashProtocolDatabase(ctx context.Context) *pgxpool.Pool {
	_, err := this.Pool.Exec(ctx, `DROP DATABASE IF EXISTS `+hashProtocolDatabase+` WITH (FORCE)`)
	this.Require().NoError(err)

	_, err = this.Pool.Exec(ctx, `CREATE DATABASE `+hashProtocolDatabase)
	this.Require().NoError(err)

	config := this.Pool.Config()
	config.ConnConfig.Database = hashProtocolDatabase

	pool, err := pgxpool.NewWithConfig(ctx, config)
	this.Require().NoError(err)

	suitePool := this.Pool
	this.T().Cleanup(func() {
		pool.Close()
		_, err := suitePool.Exec(ctx, `DROP DATABASE `+hashProtocolDatabase+` WITH (FORCE)`)
		this.Require().NoError(err)
	})

	return pool
}

func (this *IntegrationSuite) serveHashProtocolDatabase(pool *pgxpool.Pool) {
	connection := pool.Config().ConnConfig
	this.T().Setenv("BIDIREKT_DATABASE_URL", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		connection.User, connection.Password, connection.Host, connection.Port, connection.Database))

	served, err := internal.Run()
	this.Require().NoError(err)

	suiteComponents, suitePool, suiteWriterToken := this.Components, this.Pool, this.writerToken
	this.T().Cleanup(func() {
		served.Pool.Close()
		this.Components, this.Pool, this.writerToken = suiteComponents, suitePool, suiteWriterToken
	})

	this.Components, this.Pool = served, served.Pool
	this.createActiveUser(suiteWriterEmail, model.PermissionWrite, suiteWriterPassword)
	this.writerToken = this.signInToApi(suiteWriterEmail, suiteWriterPassword)
}
