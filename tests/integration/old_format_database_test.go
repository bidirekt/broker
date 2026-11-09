package integration_test

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"testing/fstest"

	"github.com/bidirekt/broker/internal"
	"github.com/bidirekt/broker/internal/model"
	"github.com/bidirekt/broker/migrations"
	"github.com/bidirekt/broker/pkg/migrator"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	oldFormatDatabase          = "old_format_resources"
	oldFormatMigrationsTable   = "public.schema_migrations"
	protocolResourcesMigration = "20261109120000_protocol_resources.sql"
)

// Written by the broker before resources were split by protocol: petstore_api 1.0.0
// deployed in production, petstore_web 1.0.0 published and checked against it.
const oldFormatPetstoreData = `
INSERT INTO participants (id, name) VALUES
  (1, 'petstore_api'),
  (2, 'petstore_web');
INSERT INTO contracts (id, participant_id, checksum) VALUES
  (1, 1, 'c987cd7ac94f23f7803cd0e923447e2ad69a40dc63aa3162d084b72e81a556c7'),
  (2, 2, '614db70fbf25d75b0677f01e2bae804ee3edce350495807d81c4e98b64c0bf78');
INSERT INTO environments (id, name) VALUES
  (1, 'production');
INSERT INTO compatibility_checks (id, participant_id, contract_id, version, environment_id, deployable, created_at, report, environment_name) VALUES
  (1, 2, 2, '1.0.0', 1, false, '2026-11-09 21:43:53.1089+00', '{"petstore_api": {"endpoints": {"/pets": {"get": {"200": [{"role": "consumer", "reason": "property_missing_in_provider", "details": {"property": "$[].photoUrl", "consumerName": "petstore_web", "propertyType": "string", "providerName": "petstore_api", "consumerField": "Pet.photoUrl"}}], "404": [{"role": "consumer", "reason": "provider_resource_interaction_not_found", "details": {"providerStatuses": "200"}}]}, "post": {"201": [{"role": "consumer", "reason": "property_missing_in_provider", "details": {"property": "$.photoUrl", "consumerName": "petstore_web", "propertyType": "string", "providerName": "petstore_api", "consumerField": "Pet.photoUrl"}}], "request": [{"role": "consumer", "reason": "property_type_mismatch", "details": {"property": "$.name", "consumerName": "petstore_web", "providerName": "petstore_api", "consumerField": "NewPet.name", "providerField": "NewPet.name", "consumerPropertyType": "integer", "providerPropertyType": "string"}}]}}}, "deployable": false, "participantVersion": "1.0.0"}}', 'production');
INSERT INTO compatibility_verdicts (contract_id_one, contract_id_two, breaks) VALUES
  (1, 2, '[{"method": "post", "reason": "property_type_mismatch", "details": {"property": "$.name", "consumerName": "petstore_web", "providerName": "petstore_api", "consumerField": "NewPet.name", "providerField": "NewPet.name", "consumerPropertyType": "integer", "providerPropertyType": "string"}, "endpoint": "/pets", "interaction": "request"}, {"method": "post", "reason": "property_missing_in_provider", "details": {"property": "$.photoUrl", "consumerName": "petstore_web", "propertyType": "string", "providerName": "petstore_api", "consumerField": "Pet.photoUrl"}, "endpoint": "/pets", "interaction": "201"}, {"method": "get", "reason": "property_missing_in_provider", "details": {"property": "$[].photoUrl", "consumerName": "petstore_web", "propertyType": "string", "providerName": "petstore_api", "consumerField": "Pet.photoUrl"}, "endpoint": "/pets", "interaction": "200"}]');
INSERT INTO compatibility_check_results (id, check_id, counterpart_name, counterpart_participant_id, counterpart_version, verdict_contract_id_one, verdict_contract_id_two, deployable, role) VALUES
  (1, 1, 'petstore_api', 1, '1.0.0', 1, 2, false, 'provider');
INSERT INTO contract_versions (id, participant_id, version, contract_id, contract_content) VALUES
  (1, 1, '1.0.0', 1, '[{"source":"api.yaml","content":"provides:\n  rest:\n    /pets:\n      get:\n        responses:\n          200: Pets\n      post:\n        request: NewPet\n        responses:\n          201: Pet\nschemas:\n  Pets:\n    type: array\n    items: { ref: Pet }\n  Pet:\n    type: object\n    properties:\n      id: { type: string }\n      name: { type: string }\n  NewPet:\n    type: object\n    properties:\n      name: { type: string }\n"}]'),
  (2, 2, '1.0.0', 2, '[{"source":"web.yaml","content":"consumes:\n  petstore_api:\n    rest:\n      /pets:\n        get:\n          responses:\n            200: Pets\n            404: ~\n        post:\n          request: NewPet\n          responses:\n            201: Pet\nschemas:\n  Pets:\n    type: array\n    items: { ref: Pet }\n  Pet:\n    type: object\n    properties:\n      id: { type: string }\n      name: { type: string }\n      photoUrl: { type: string }\n  NewPet:\n    type: object\n    properties:\n      name: { type: integer }\n"}]');
INSERT INTO current_deployments (participant_id, environment_id, version, contract_id, deployed_at) VALUES
  (1, 1, '1.0.0', 1, '2026-11-09 21:43:53.086605+00');
INSERT INTO deployments (id, participant_id, version, environment_id, rollback, deployed_at) VALUES
  (1, 1, '1.0.0', 1, false, '2026-11-09 21:43:53.086605+00');
INSERT INTO schemas (id, participant_id, name) VALUES
  (1, 1, 'Pets'),
  (2, 1, 'Pet'),
  (3, 1, 'NewPet'),
  (4, 2, 'Pets'),
  (5, 2, 'Pet'),
  (6, 2, 'NewPet');
INSERT INTO fields (id, schema_id, path) VALUES
  (1, 1, ''),
  (2, 1, '[]'),
  (3, 2, ''),
  (4, 2, 'id'),
  (5, 2, 'name'),
  (6, 3, ''),
  (7, 3, 'name'),
  (8, 4, ''),
  (9, 4, '[]'),
  (10, 5, ''),
  (11, 5, 'photoUrl'),
  (12, 5, 'id'),
  (13, 5, 'name'),
  (14, 6, ''),
  (15, 6, 'name');
INSERT INTO field_versions (id, field_id, contract_id, type, optional, nullable, ref, change_type) VALUES
  (1, 1, 1, 'array', false, false, NULL, 'added'),
  (2, 2, 1, NULL, false, false, 'Pet', 'added'),
  (3, 3, 1, 'object', false, false, NULL, 'added'),
  (4, 4, 1, 'string', false, false, NULL, 'added'),
  (5, 5, 1, 'string', false, false, NULL, 'added'),
  (6, 6, 1, 'object', false, false, NULL, 'added'),
  (7, 7, 1, 'string', false, false, NULL, 'added'),
  (8, 8, 2, 'array', false, false, NULL, 'added'),
  (9, 9, 2, NULL, false, false, 'Pet', 'added'),
  (10, 10, 2, 'object', false, false, NULL, 'added'),
  (11, 11, 2, 'string', false, false, NULL, 'added'),
  (12, 12, 2, 'string', false, false, NULL, 'added'),
  (13, 13, 2, 'string', false, false, NULL, 'added'),
  (14, 14, 2, 'object', false, false, NULL, 'added'),
  (15, 15, 2, 'integer', false, false, NULL, 'added');
INSERT INTO resources (id, participant_id, direction, interaction, consumed_provider, endpoint, method, response_status_code, provider_hash, consumer_hash) VALUES
  (1, 1, 'provides', 'rest_response', NULL, '/pets', 'get', '200', '4186fd903c086826655928f1aac69b6a09dd339d11d370c19df275c56878aca4', NULL),
  (2, 1, 'provides', 'rest_request', NULL, '/pets', 'post', NULL, '821227ce1e0880384ff4d8d8891cafb957ebeadfde79a4df94ab3874491beedf', NULL),
  (3, 1, 'provides', 'rest_response', NULL, '/pets', 'post', '201', '5676ad94449428cf85488fa9c7a59c6edc43b6937e83d6224f4055075bc69b94', NULL),
  (4, 2, 'consumes', 'rest_response', 'petstore_api', '/pets', 'get', '404', 'dec160001823a4ddfe433a804e67b09e578c1b189a6128e0044bfa063cc4f23e', 'f1ce5f26dae083ddd7694e77b047e3e076da160e3d6cfa0971017e1b5ec88ada'),
  (5, 2, 'consumes', 'rest_request', 'petstore_api', '/pets', 'post', NULL, '821227ce1e0880384ff4d8d8891cafb957ebeadfde79a4df94ab3874491beedf', '66c18366e53a4ae1638906821bab8b81376821afe29c3f1ed448dfca299e427a'),
  (6, 2, 'consumes', 'rest_response', 'petstore_api', '/pets', 'post', '201', '5676ad94449428cf85488fa9c7a59c6edc43b6937e83d6224f4055075bc69b94', '41b9b6619aa979d50726b49831faa14b15711a3a9b4b832fd89dd788bb1aa6d0'),
  (7, 2, 'consumes', 'rest_response', 'petstore_api', '/pets', 'get', '200', '4186fd903c086826655928f1aac69b6a09dd339d11d370c19df275c56878aca4', '0c6429446ddbdc49fcbc14dd1f160a345f7d30ce6bbd63660f019df6517b2243');
INSERT INTO resource_bodies (id, resource_id, contract_id, source, schema_id) VALUES
  (1, 1, 1, 'api.yaml', 1),
  (2, 2, 1, 'api.yaml', 3),
  (3, 3, 1, 'api.yaml', 2),
  (4, 6, 2, 'web.yaml', 5),
  (5, 7, 2, 'web.yaml', 4),
  (6, 4, 2, 'web.yaml', NULL),
  (7, 5, 2, 'web.yaml', 6);
INSERT INTO resource_versions (id, resource_id, contract_id, change_type) VALUES
  (1, 1, 1, 'added'),
  (2, 2, 1, 'added'),
  (3, 3, 1, 'added'),
  (4, 4, 2, 'added'),
  (5, 5, 2, 'added'),
  (6, 6, 2, 'added'),
  (7, 7, 2, 'added');
INSERT INTO schema_versions (id, schema_id, contract_id, change_type) VALUES
  (1, 1, 1, 'added'),
  (2, 2, 1, 'added'),
  (3, 3, 1, 'added'),
  (4, 4, 2, 'added'),
  (5, 5, 2, 'added'),
  (6, 6, 2, 'added');
SELECT pg_catalog.setval('compatibility_check_results_id_seq', 1, true);
SELECT pg_catalog.setval('compatibility_checks_id_seq', 1, true);
SELECT pg_catalog.setval('contract_versions_id_seq', 2, true);
SELECT pg_catalog.setval('contracts_id_seq', 2, true);
SELECT pg_catalog.setval('deployments_id_seq', 1, true);
SELECT pg_catalog.setval('environments_id_seq', 1, true);
SELECT pg_catalog.setval('field_versions_id_seq', 15, true);
SELECT pg_catalog.setval('fields_id_seq', 15, true);
SELECT pg_catalog.setval('participants_id_seq', 2, true);
SELECT pg_catalog.setval('resource_bodies_id_seq', 7, true);
SELECT pg_catalog.setval('resource_versions_id_seq', 7, true);
SELECT pg_catalog.setval('resources_id_seq', 7, true);
SELECT pg_catalog.setval('schema_versions_id_seq', 6, true);
SELECT pg_catalog.setval('schemas_id_seq', 6, true);
`

const oldFormatPetstoreCanIDeploy = `{"message":"contract checked successfully","participant":"petstore_web","version":"1.0.0","environment":"production","deployable":false,"results":{"petstore_api":{"deployable":false,"participantVersion":"1.0.0","endpoints":{"/pets":{"get":{"200":[{"reason":"property_missing_in_provider","role":"consumer","details":{"consumerField":"Pet.photoUrl","consumerName":"petstore_web","property":"$[].photoUrl","propertyType":"string","providerName":"petstore_api"}}],"404":[{"reason":"provider_resource_interaction_not_found","role":"consumer","details":{"providerStatuses":"200"}}]},"post":{"201":[{"reason":"property_missing_in_provider","role":"consumer","details":{"consumerField":"Pet.photoUrl","consumerName":"petstore_web","property":"$.photoUrl","propertyType":"string","providerName":"petstore_api"}}],"request":[{"reason":"property_type_mismatch","role":"consumer","details":{"consumerField":"NewPet.name","consumerName":"petstore_web","consumerPropertyType":"integer","property":"$.name","providerField":"NewPet.name","providerName":"petstore_api","providerPropertyType":"string"}}]}}}}}}`

func (this *IntegrationSuite) TestOldFormatDatabase_AnswersTheSameAfterTheProtocolResourcesMigration() {
	ctx := context.Background()

	pool := this.openOldFormatDatabase(ctx)

	this.Require().NoError(migrator.New(pool, this.migrationsBeforeProtocolResources(), oldFormatMigrationsTable).Migrate())
	_, err := pool.Exec(ctx, oldFormatPetstoreData)
	this.Require().NoError(err)

	this.serveOldFormatDatabase(pool)

	status, body := this.post("/api/can-i-deploy", `{"participant":"petstore_web","version":"1.0.0","environment":"production"}`)
	this.Equal(http.StatusOK, status)
	this.Equal(oldFormatPetstoreCanIDeploy, body)

	status, _, page := this.getPage("/checks/1", this.signInAsAdmin())
	this.Require().Equal(http.StatusOK, status, page)
	for _, line := range []string{
		`<div><b>3</b><span>breaks</span></div>`,
		`<div><b>1 of 1</b><span>counterpart breaks</span></div>`,
		`<div><b>3</b><span>places in the contracts</span></div>`,
		`no · 3 breaks</span></summary>`,
		`<div class="ep-title"><span class="meth">GET</span><span>/pets</span></div>`,
		`<p class="ix-name">response 404</p>`,
		`<span><b class="name">petstore_web</b> reads response 404, but <b class="name">petstore_api</b> answers only 200</span>`,
		`<p class="muted">petstore_api 1.0.0, deployed in production, declares the endpoint but not this interaction (declared: 200)</p>`,
		`<div class="fixes"><div class="fix self"><span class="side">Fix in <b>petstore_web</b></span><span>read response 200</span></div><div class="fix other"><span class="side">or fix in <b>petstore_api</b></span><span>answer 404</span></div></div>`,
		`<div class="ep-title"><span class="prop">FIELD</span><span>NewPet.name</span><span class="badge">1 place</span></div>`,
		`<span><b class="name">petstore_web</b> sends it as <code>integer</code>, but <b class="name">petstore_api</b> expects <code>string</code></span>`,
		`<div class="fixes"><div class="fix self"><span class="side">Fix in <b>petstore_web</b></span><span>send string</span></div><div class="fix other"><span class="side">or fix in <b>petstore_api</b></span><span>accept integer there</span></div></div>`,
		`<li><span class="meth">POST</span><span>/pets</span><span class="at">request</span><span>at &#34;$.name&#34;</span><span class="cf">(petstore_api: NewPet.name)</span></li>`,
		`<div class="ep-title"><span class="prop">FIELD</span><span>Pet.photoUrl</span><span class="badge">2 places</span></div>`,
		`<span><b class="name">petstore_web</b> reads it, but <b class="name">petstore_api</b> doesn&#39;t provide it</span>`,
		`<li><span class="meth">GET</span><span>/pets</span><span class="at">response 200</span><span>at &#34;$[].photoUrl&#34;</span></li>`,
		`<li><span class="meth">POST</span><span>/pets</span><span class="at">response 201</span><span>at &#34;$.photoUrl&#34;</span></li>`,
	} {
		this.Contains(page, line)
	}
}

func (this *IntegrationSuite) migrationsBeforeProtocolResources() fs.FS {
	entries, err := fs.ReadDir(migrations.FS, ".")
	this.Require().NoError(err)

	earlier := fstest.MapFS{}
	for _, entry := range entries {
		if entry.Name() >= protocolResourcesMigration {
			continue
		}
		contents, err := fs.ReadFile(migrations.FS, entry.Name())
		this.Require().NoError(err)
		earlier[entry.Name()] = &fstest.MapFile{Data: contents}
	}

	return earlier
}

func (this *IntegrationSuite) openOldFormatDatabase(ctx context.Context) *pgxpool.Pool {
	_, err := this.Pool.Exec(ctx, `DROP DATABASE IF EXISTS `+oldFormatDatabase+` WITH (FORCE)`)
	this.Require().NoError(err)

	_, err = this.Pool.Exec(ctx, `CREATE DATABASE `+oldFormatDatabase)
	this.Require().NoError(err)

	config := this.Pool.Config()
	config.ConnConfig.Database = oldFormatDatabase

	pool, err := pgxpool.NewWithConfig(ctx, config)
	this.Require().NoError(err)

	suitePool := this.Pool
	this.T().Cleanup(func() {
		pool.Close()
		_, err := suitePool.Exec(ctx, `DROP DATABASE `+oldFormatDatabase+` WITH (FORCE)`)
		this.Require().NoError(err)
	})

	return pool
}

// serveOldFormatDatabase starts the broker on the old database, which migrates it, and points
// the suite's requests at that broker until the test ends.
func (this *IntegrationSuite) serveOldFormatDatabase(pool *pgxpool.Pool) {
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
	this.createActiveUser(suiteAdminEmail, model.PermissionAdmin, suiteAdminSecret)
}
