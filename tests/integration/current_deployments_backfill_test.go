package integration_test

import (
	"context"
	"io/fs"
	"time"

	"github.com/bidirekt/broker/migrations"
	"github.com/bidirekt/broker/pkg/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	backfillDatabase       = "current_deployments_backfill"
	backfillMigration      = "20261104120000_web_ui_data.sql"
	backfillMigrationTable = "public.schema_migrations"
)

type migrationsWithout struct {
	fs.FS
	hidden string
}

func (this migrationsWithout) Open(name string) (fs.File, error) {
	if name == this.hidden {
		return nil, fs.ErrNotExist
	}

	return this.FS.Open(name)
}

func (this migrationsWithout) ReadDir(name string) ([]fs.DirEntry, error) {
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

type backfilledCurrentDeployment struct {
	participant string
	environment string
	version     string
	contractID  int64
	deployedAt  time.Time
}

func (this *IntegrationSuite) openBackfillDatabase(ctx context.Context) *pgxpool.Pool {
	_, err := this.Pool.Exec(ctx, `DROP DATABASE IF EXISTS `+backfillDatabase+` WITH (FORCE)`)
	this.Require().NoError(err)

	_, err = this.Pool.Exec(ctx, `CREATE DATABASE `+backfillDatabase)
	this.Require().NoError(err)

	config := this.Pool.Config()
	config.ConnConfig.Database = backfillDatabase

	pool, err := pgxpool.NewWithConfig(ctx, config)
	this.Require().NoError(err)

	return pool
}

func (this *IntegrationSuite) dropBackfillDatabase(ctx context.Context, pool *pgxpool.Pool) {
	pool.Close()

	_, err := this.Pool.Exec(ctx, `DROP DATABASE `+backfillDatabase+` WITH (FORCE)`)
	this.Require().NoError(err)
}

func (this *IntegrationSuite) backfillSeed(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int64 {
	var id int64
	this.Require().NoError(pool.QueryRow(ctx, query, args...).Scan(&id))

	return id
}

func (this *IntegrationSuite) TestCurrentDeploymentsMigration_BackfillsTheNewestDeploymentOfEachPair() {
	ctx := context.Background()

	pool := this.openBackfillDatabase(ctx)
	defer this.dropBackfillDatabase(ctx, pool)

	before := migrationsWithout{FS: migrations.FS, hidden: backfillMigration}
	this.Require().NoError(migrator.New(pool, before, backfillMigrationTable).Migrate())

	var tableExists bool
	this.Require().NoError(pool.QueryRow(ctx, `SELECT to_regclass('current_deployments') IS NOT NULL`).Scan(&tableExists))
	this.Require().False(tableExists, "the hidden migration must not run before the deployments are seeded")

	apiID := this.backfillSeed(ctx, pool, `INSERT INTO participants (name) VALUES ('api') RETURNING id`)
	frontID := this.backfillSeed(ctx, pool, `INSERT INTO participants (name) VALUES ('front') RETURNING id`)
	productionID := this.backfillSeed(ctx, pool, `INSERT INTO environments (name) VALUES ('production') RETURNING id`)
	stagingID := this.backfillSeed(ctx, pool, `INSERT INTO environments (name) VALUES ('staging') RETURNING id`)

	contractIDs := map[int64]map[string]int64{apiID: {}, frontID: {}}
	for _, participantID := range []int64{apiID, frontID} {
		for _, version := range []string{"v1", "v2"} {
			contractID := this.backfillSeed(ctx, pool,
				`INSERT INTO contracts (participant_id, checksum) VALUES ($1, $2) RETURNING id`,
				participantID, version,
			)
			this.backfillSeed(ctx, pool,
				`INSERT INTO contract_versions (participant_id, version, contract_id, contract_content)
				 VALUES ($1, $2, $3, '') RETURNING id`,
				participantID, version, contractID,
			)
			contractIDs[participantID][version] = contractID
		}
	}

	day := func(n int) time.Time { return time.Date(2026, 5, n, 0, 0, 0, 0, time.UTC) }

	deployments := []struct {
		participantID int64
		environmentID int64
		version       string
		rollback      bool
		deployedAt    time.Time
	}{
		{apiID, productionID, "v1", false, day(1)},
		{apiID, productionID, "v2", false, day(2)},
		{apiID, productionID, "v1", true, day(3)},
		{apiID, stagingID, "v2", false, day(4)},
		{frontID, productionID, "v1", false, day(5)},
		{frontID, stagingID, "v1", false, day(6)},
		{frontID, stagingID, "v2", false, day(7)},
	}
	for _, deployment := range deployments {
		this.backfillSeed(ctx, pool,
			`INSERT INTO deployments (participant_id, environment_id, version, rollback, deployed_at)
			 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			deployment.participantID, deployment.environmentID, deployment.version, deployment.rollback, deployment.deployedAt,
		)
	}

	this.Require().NoError(migrator.New(pool, migrations.FS, backfillMigrationTable).Migrate())

	rows, err := pool.Query(ctx,
		`SELECT p.name, e.name, cd.version, cd.contract_id, cd.deployed_at
		 FROM current_deployments cd
		 JOIN participants p ON p.id = cd.participant_id
		 JOIN environments e ON e.id = cd.environment_id
		 ORDER BY p.name COLLATE "C", e.name COLLATE "C"`,
	)
	this.Require().NoError(err)

	got, err := pgx.AppendRows(make([]backfilledCurrentDeployment, 0), rows, func(row pgx.CollectableRow) (backfilledCurrentDeployment, error) {
		var current backfilledCurrentDeployment
		err := row.Scan(&current.participant, &current.environment, &current.version, &current.contractID, &current.deployedAt)
		current.deployedAt = current.deployedAt.UTC()

		return current, err
	})
	this.Require().NoError(err)

	this.Equal([]backfilledCurrentDeployment{
		{"api", "production", "v1", contractIDs[apiID]["v1"], day(3)},
		{"api", "staging", "v2", contractIDs[apiID]["v2"], day(4)},
		{"front", "production", "v1", contractIDs[frontID]["v1"], day(5)},
		{"front", "staging", "v2", contractIDs[frontID]["v2"], day(7)},
	}, got)
}
