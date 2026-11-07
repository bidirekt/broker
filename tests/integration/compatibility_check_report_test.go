package integration_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/bidirekt/broker/migrations"
	"github.com/bidirekt/broker/pkg/migrator"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	checkReportBackfillDatabase       = "store_check_reports_backfill"
	checkReportMigration              = "20261104120000_web_ui_data.sql"
	checkReportBackfillMigrationTable = "public.schema_migrations"
)

const checkReportOrdersV1YAML = `provides:
  rest:
    /orders:
      get:
        responses:
          204: ~
consumes:
  orders:
    rest:
      /orders:
        get:
          responses:
            204: ~
`

const checkReportOrdersV2YAML = `provides:
  rest:
    /orders:
      get:
        responses:
          204: ~
consumes:
  orders:
    rest:
      /orders:
        get:
          responses:
            204: ~
  billing:
    rest:
      /invoices:
        get:
          responses:
            204: ~
  ghost:
    rest:
      /ghosts:
        get:
          responses:
            200: ~
`

const checkReportBillingYAML = `provides:
  rest:
    /invoices:
      get:
        responses:
          204: ~
`

const checkReportFrontYAML = `consumes:
  orders:
    rest:
      /orders:
        get:
          responses:
            204: ~
`

type checkReportMigrationsWithout struct {
	fs.FS
	hidden string
}

func (this checkReportMigrationsWithout) Open(name string) (fs.File, error) {
	if name == this.hidden {
		return nil, fs.ErrNotExist
	}

	return this.FS.Open(name)
}

func (this checkReportMigrationsWithout) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(this.FS, name)
	if err != nil {
		return nil, err
	}

	visible := make([]fs.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() != this.hidden {
			visible = append(visible, entry)
		}
	}

	return visible, nil
}

func (this *IntegrationSuite) mustPostForCheckReport(path, body string) {
	status, response := this.post(path, body)
	this.Require().Equalf(http.StatusOK, status, "POST %s: %s", path, response)
}

func (this *IntegrationSuite) setupForCheckReport() string {
	this.mustPostForCheckReport("/api/environments", `{"environment":"production"}`)
	for _, participant := range []string{"orders", "billing", "front"} {
		this.mustPostForCheckReport("/api/participants", `{"participant":"`+participant+`"}`)
	}

	publishAndDeploy := func(participant, version, content string) {
		this.mustPostForCheckReport("/api/contracts",
			this.publishBody(participant, version, contractFragment{"api.yaml", content}))
		this.mustPostForCheckReport("/api/deployments",
			`{"participant":"`+participant+`","version":"`+version+`","environment":"production"}`)
	}

	publishAndDeploy("orders", "v1", checkReportOrdersV1YAML)
	publishAndDeploy("billing", "v1", checkReportBillingYAML)
	publishAndDeploy("front", "v1", checkReportFrontYAML)

	this.mustPostForCheckReport("/api/contracts",
		this.publishBody("orders", "v2", contractFragment{"api.yaml", checkReportOrdersV2YAML}))

	status, body := this.post("/api/can-i-deploy", `{"participant":"orders","version":"v2","environment":"production"}`)
	this.Require().Equalf(http.StatusOK, status, body)

	return body
}

func (this *IntegrationSuite) TestCheckReport_StoresTheResponseResultsAndTheEnvironmentName() {
	body := this.setupForCheckReport()

	var response struct {
		Results json.RawMessage `json:"results"`
	}
	this.Require().NoError(json.Unmarshal([]byte(body), &response))

	_, err := this.Pool.Exec(context.Background(), `UPDATE environments SET name = 'live'`)
	this.Require().NoError(err)

	var report, environmentName string
	this.Require().NoError(this.Pool.QueryRow(context.Background(),
		`SELECT report::text, environment_name FROM compatibility_checks`).Scan(&report, &environmentName))

	this.JSONEq(string(response.Results), report)
	this.Equal("production", environmentName)
}

func (this *IntegrationSuite) TestCheckReport_StoresTheRoleOfEveryCounterpart() {
	this.setupForCheckReport()

	rows, err := this.Pool.Query(context.Background(),
		`SELECT counterpart_name, deployable, role FROM compatibility_check_results`)
	this.Require().NoError(err)
	defer rows.Close()

	type storedRole struct {
		deployable bool
		role       string
	}

	got := map[string]storedRole{}
	for rows.Next() {
		var name string
		var stored storedRole
		this.Require().NoError(rows.Scan(&name, &stored.deployable, &stored.role))
		got[name] = stored
	}
	this.Require().NoError(rows.Err())

	this.Equal(map[string]storedRole{
		"billing": {deployable: true, role: "provider"},
		"ghost":   {deployable: false, role: "provider"},
		"front":   {deployable: true, role: "consumer"},
		"orders":  {deployable: true, role: "both"},
	}, got)
}

func (this *IntegrationSuite) TestCheckReport_RejectsAnUnknownRole() {
	this.setupForCheckReport()

	_, err := this.Pool.Exec(context.Background(), `UPDATE compatibility_check_results SET role = 'observer'`)
	this.Require().Error(err)
}

func (this *IntegrationSuite) openCheckReportBackfillDatabase(ctx context.Context) *pgxpool.Pool {
	_, err := this.Pool.Exec(ctx, `DROP DATABASE IF EXISTS `+checkReportBackfillDatabase+` WITH (FORCE)`)
	this.Require().NoError(err)

	_, err = this.Pool.Exec(ctx, `CREATE DATABASE `+checkReportBackfillDatabase)
	this.Require().NoError(err)

	config := this.Pool.Config()
	config.ConnConfig.Database = checkReportBackfillDatabase

	pool, err := pgxpool.NewWithConfig(ctx, config)
	this.Require().NoError(err)

	return pool
}

func (this *IntegrationSuite) TestCheckReportMigration_LeavesEarlierChecksNull() {
	ctx := context.Background()

	pool := this.openCheckReportBackfillDatabase(ctx)
	defer func() {
		pool.Close()
		_, err := this.Pool.Exec(ctx, `DROP DATABASE `+checkReportBackfillDatabase+` WITH (FORCE)`)
		this.Require().NoError(err)
	}()

	before := checkReportMigrationsWithout{FS: migrations.FS, hidden: checkReportMigration}
	this.Require().NoError(migrator.New(pool, before, checkReportBackfillMigrationTable).Migrate())

	seed := func(query string, args ...any) int64 {
		var id int64
		this.Require().NoError(pool.QueryRow(ctx, query, args...).Scan(&id))
		return id
	}

	participantID := seed(`INSERT INTO participants (name) VALUES ('front') RETURNING id`)
	environmentID := seed(`INSERT INTO environments (name) VALUES ('production') RETURNING id`)
	contractID := seed(`INSERT INTO contracts (participant_id, checksum) VALUES ($1, 'v1') RETURNING id`, participantID)
	checkID := seed(
		`INSERT INTO compatibility_checks (participant_id, contract_id, version, environment_id, deployable)
		 VALUES ($1, $2, 'v1', $3, false) RETURNING id`,
		participantID, contractID, environmentID,
	)
	seed(
		`INSERT INTO compatibility_check_results (check_id, counterpart_name, deployable)
		 VALUES ($1, 'orders', false) RETURNING id`,
		checkID,
	)

	this.Require().NoError(migrator.New(pool, migrations.FS, checkReportBackfillMigrationTable).Migrate())

	var report, environmentName, role *string
	this.Require().NoError(pool.QueryRow(ctx,
		`SELECT c.report::text, c.environment_name, r.role
		   FROM compatibility_checks c
		   JOIN compatibility_check_results r ON r.check_id = c.id`).Scan(&report, &environmentName, &role))

	this.Nil(report)
	this.Nil(environmentName)
	this.Nil(role)
}
