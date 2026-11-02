package integration_test

import "context"

func (this *IntegrationSuite) TestDatabaseSchema_RefusesASecondAdmin() {
	_, err := this.Pool.Exec(context.Background(),
		`INSERT INTO users (email, permission, password_hash) VALUES ('second-admin@x.com', 'admin', 'some-hash')`,
	)

	this.ErrorContains(err, `duplicate key value violates unique constraint "users_one_admin"`)
}

func (this *IntegrationSuite) TestDatabaseSchema_RefusesASessionWithoutAUser() {
	_, err := this.Pool.Exec(context.Background(),
		`INSERT INTO sessions (id_hash, csrf_token) VALUES ($1, 'no-user-csrf')`, []byte("no-user-session"),
	)

	this.ErrorContains(err, `null value in column "user_id" of relation "sessions" violates not-null constraint`)
}
