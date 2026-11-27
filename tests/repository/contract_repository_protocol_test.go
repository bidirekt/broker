package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bidirekt/broker/internal/model"
	restmodel "github.com/bidirekt/broker/internal/model/protocols/rest"
	"github.com/bidirekt/broker/internal/repository"
	restrepository "github.com/bidirekt/broker/internal/repository/protocols/rest"
	"github.com/bidirekt/broker/migrations"
	"github.com/bidirekt/broker/pkg/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type ContractRepositoryProtocolSuite struct {
	suite.Suite
	container *postgres.PostgresContainer
	pool      *pgxpool.Pool
}

func TestContractRepositoryProtocolSuite(t *testing.T) {
	suite.Run(t, new(ContractRepositoryProtocolSuite))
}

func (this *ContractRepositoryProtocolSuite) SetupSuite() {
	ctx := context.Background()

	container, err := postgres.Run(
		ctx, "postgres:18.6-alpine",
		postgres.WithDatabase("contracttests"),
		postgres.WithUsername("contracttests"),
		postgres.WithPassword("s3cr3t"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	this.Require().NoError(err)
	this.container = container

	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	this.Require().NoError(err)

	pool, err := pgxpool.New(ctx, connectionString)
	this.Require().NoError(err)
	this.pool = pool

	this.Require().NoError(migrator.New(pool, migrations.FS, "public.schema_migrations").Migrate())
}

func (this *ContractRepositoryProtocolSuite) TearDownSuite() {
	if this.pool != nil {
		this.pool.Close()
	}
	if this.container != nil {
		_ = this.container.Terminate(context.Background())
	}
}

func (this *ContractRepositoryProtocolSuite) restRepository() *repository.ContractRepository {
	return repository.NewContractRepository(this.pool, repository.Protocols{Tables: []repository.ProtocolTable{restrepository.New()}})
}

func (this *ContractRepositoryProtocolSuite) uploadedPetsContract(participantName string) *model.UploadedContract {
	var participantID int64
	this.Require().NoError(this.pool.QueryRow(context.Background(),
		`INSERT INTO participants (name) VALUES ($1) RETURNING id`, participantName).Scan(&participantID))

	contract := model.NewUploadedContract(participantID, participantName, "1",
		[]model.ContractFile{{Source: "api.yaml", Content: "provides: {}"}})

	for _, resource := range []model.UploadedResource{
		{Resource: model.Resource{
			Direction: model.Provides,
			Protocol:  restmodel.Protocol,
			Body:      model.Output,
			Identity:  restmodel.Identity{Endpoint: "/pets", Method: "get", Status: "200"},
		}},
		{Resource: model.Resource{
			Direction: model.Provides,
			Protocol:  restmodel.Protocol,
			Body:      model.Input,
			Identity:  restmodel.Identity{Endpoint: "/pets", Method: "post"},
		}},
	} {
		this.Require().NoError(contract.AddResource(&resource))
	}

	return contract
}

func (this *ContractRepositoryProtocolSuite) publish(repo *repository.ContractRepository, contract *model.UploadedContract) {
	result := repo.Publish(
		context.Background(),
		contract,
		func(*model.PersistedContract) model.ContractDiff { return model.ContractDiff{} },
		func([]model.ContractFile) bool { return true },
	)
	this.Require().Equal(repository.Published, result.Outcome)
}

func (this *ContractRepositoryProtocolSuite) insertLiveResource(contract *model.UploadedContract, protocol string, providerHash string) int64 {
	ctx := context.Background()

	var resourceID int64
	this.Require().NoError(this.pool.QueryRow(ctx,
		`INSERT INTO resources (participant_id, direction, protocol, body_role, provider_hash)
		 VALUES ($1, 'provides', $2, 'output', $3) RETURNING id`,
		contract.ParticipantID, protocol, providerHash).Scan(&resourceID))

	_, err := this.pool.Exec(ctx,
		`INSERT INTO resource_versions (resource_id, contract_id, change_type) VALUES ($1, $2, 'added')`,
		resourceID, contract.ID)
	this.Require().NoError(err)

	return resourceID
}

func (this *ContractRepositoryProtocolSuite) TestPublishWritesEachResourceToResourcesAndRestResources() {
	contract := this.uploadedPetsContract("pets_written")
	this.publish(this.restRepository(), contract)

	type writtenRow struct {
		Direction          string
		Protocol           string
		BodyRole           string
		RestProtocol       string
		Endpoint           string
		Method             string
		ResponseStatusCode *string
	}

	rows, err := this.pool.Query(context.Background(), `
		SELECT r.direction, r.protocol, r.body_role, rr.protocol, rr.endpoint, rr.method, rr.response_status_code
		FROM resources r
		JOIN rest_resources rr ON rr.resource_id = r.id
		WHERE r.participant_id = $1
		ORDER BY rr.method`, contract.ParticipantID)
	this.Require().NoError(err)
	written, err := pgx.CollectRows(rows, pgx.RowToStructByPos[writtenRow])
	this.Require().NoError(err)

	status := "200"
	this.Equal([]writtenRow{
		{Direction: "provides", Protocol: "rest", BodyRole: "output", RestProtocol: "rest", Endpoint: "/pets", Method: "get", ResponseStatusCode: &status},
		{Direction: "provides", Protocol: "rest", BodyRole: "input", RestProtocol: "rest", Endpoint: "/pets", Method: "post", ResponseStatusCode: nil},
	}, written)
}

func (this *ContractRepositoryProtocolSuite) TestLoadedResourcesCarryTheirProtocolBodyRoleAndIdentity() {
	uploaded := this.uploadedPetsContract("pets_loaded")
	repo := this.restRepository()
	this.publish(repo, uploaded)

	contract, found := repo.GetContractByNameAndVersion(context.Background(), "pets_loaded", "1")
	this.Require().True(found)

	loaded := make(map[string]model.Resource, len(contract.Resources))
	for hash, resource := range contract.Resources {
		loaded[hash] = resource.Resource
	}

	expected := make(map[string]model.Resource, len(uploaded.Resources))
	for hash, resource := range uploaded.Resources {
		resource.Bodies = []model.ResourceBody{}
		expected[hash] = resource.Resource
	}

	this.Equal(expected, loaded)
}

func (this *ContractRepositoryProtocolSuite) TestPublishingAProtocolWithoutARegisteredTableFails() {
	contract := this.uploadedPetsContract("pets_unregistered_publish")
	repo := repository.NewContractRepository(this.pool, repository.Protocols{})

	defer func() {
		panicked, isError := recover().(error)
		this.Require().True(isError)
		this.Regexp(`^resource \d+ has protocol "rest", which has no registered table$`, panicked.Error())
	}()

	this.publish(repo, contract)
}

func (this *ContractRepositoryProtocolSuite) TestLoadingAResourceOfAnUnregisteredProtocolFails() {
	contract := this.uploadedPetsContract("pets_grpc")
	repo := this.restRepository()
	this.publish(repo, contract)
	grpcResourceID := this.insertLiveResource(contract, "grpc", "pets-grpc-hash")

	this.PanicsWithError(fmt.Sprintf(`resource %d has protocol "grpc", which has no registered table`, grpcResourceID), func() {
		repo.GetContractByNameAndVersion(context.Background(), "pets_grpc", "1")
	})
}

func (this *ContractRepositoryProtocolSuite) TestLoadingAResourceWithoutItsProtocolRowFails() {
	contract := this.uploadedPetsContract("pets_rowless")
	repo := this.restRepository()
	this.publish(repo, contract)
	rowlessResourceID := this.insertLiveResource(contract, "rest", "pets-rowless-hash")

	this.PanicsWithError(fmt.Sprintf(`resource %d has no row in its protocol table`, rowlessResourceID), func() {
		repo.GetContractByNameAndVersion(context.Background(), "pets_rowless", "1")
	})
}

type recordingExtensionStore struct {
	refusal                      model.Refusal
	refuses                      bool
	refusalAsked                 []any
	inserted                     map[int64]any
	insertedInsideTheTransaction map[int64]bool
}

func newRecordingExtensionStore() *recordingExtensionStore {
	return &recordingExtensionStore{inserted: map[int64]any{}, insertedInsideTheTransaction: map[int64]bool{}}
}

func (this *recordingExtensionStore) Protocol() model.Protocol {
	return restmodel.Protocol
}

func (this *recordingExtensionStore) RefuseContractExtension(_ context.Context, _ repository.Querier, extension any) (model.Refusal, bool) {
	this.refusalAsked = append(this.refusalAsked, extension)

	return this.refusal, this.refuses
}

func (this *recordingExtensionStore) InsertContractExtension(ctx context.Context, q repository.Querier, contractID int64, extension any) {
	var visible bool
	if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM contracts WHERE id = $1)`, contractID).Scan(&visible); err != nil {
		panic(err)
	}

	this.inserted[contractID] = extension
	this.insertedInsideTheTransaction[contractID] = visible
}

func (this *recordingExtensionStore) LoadContractExtensions(_ context.Context, _ repository.Querier, contractIDs []int64) map[int64]any {
	loaded := map[int64]any{}
	for _, id := range contractIDs {
		if extension, found := this.inserted[id]; found {
			loaded[id] = extension
		}
	}

	return loaded
}

func (this *ContractRepositoryProtocolSuite) repositoryWithExtensions(store *recordingExtensionStore) *repository.ContractRepository {
	return repository.NewContractRepository(this.pool, repository.Protocols{
		Tables:     []repository.ProtocolTable{restrepository.New()},
		Extensions: []repository.ContractExtensionStore{store},
	})
}

func (this *ContractRepositoryProtocolSuite) countContractsOf(participantID int64) int {
	var count int
	this.Require().NoError(this.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM contracts WHERE participant_id = $1`, participantID).Scan(&count))

	return count
}

func (this *ContractRepositoryProtocolSuite) TestALoadedContractCarriesTheExtensionsItWasPublishedWith() {
	store := newRecordingExtensionStore()
	repo := this.repositoryWithExtensions(store)
	uploaded := this.uploadedPetsContract("pets_extended")
	uploaded.Extensions = map[string]any{"rest": "petstore_graph"}
	this.publish(repo, uploaded)

	contract, found := repo.GetContractByNameAndVersion(context.Background(), "pets_extended", "1")
	this.Require().True(found)

	this.Equal(map[string]any{"rest": "petstore_graph"}, contract.Extensions)
	this.Equal(map[int64]bool{contract.ID: true}, store.insertedInsideTheTransaction)
}

func (this *ContractRepositoryProtocolSuite) TestAContractWithoutExtensionsIsNeitherAskedNorInserted() {
	store := newRecordingExtensionStore()
	repo := this.repositoryWithExtensions(store)
	this.publish(repo, this.uploadedPetsContract("pets_plain"))

	contract, found := repo.GetContractByNameAndVersion(context.Background(), "pets_plain", "1")
	this.Require().True(found)

	this.Nil(contract.Extensions)
	this.Empty(store.refusalAsked)
	this.Empty(store.inserted)
}

func (this *ContractRepositoryProtocolSuite) TestAVersionReusingASnapshotReusesItsExtensionWithoutInsertingIt() {
	store := newRecordingExtensionStore()
	repo := this.repositoryWithExtensions(store)
	first := this.uploadedPetsContract("pets_aliased_extension")
	first.Extensions = map[string]any{"rest": "petstore_graph"}
	this.publish(repo, first)

	second := model.NewUploadedContract(first.ParticipantID, first.ParticipantName, "2", first.Files)
	second.Resources = first.Resources
	second.Extensions = map[string]any{"rest": "petstore_graph"}
	this.publish(repo, second)

	contract, found := repo.GetContractByNameAndVersion(context.Background(), "pets_aliased_extension", "2")
	this.Require().True(found)

	this.Equal(map[string]any{"rest": "petstore_graph"}, contract.Extensions)
	this.Len(store.inserted, 1)
	this.Equal([]any{"petstore_graph", "petstore_graph"}, store.refusalAsked)
}

func (this *ContractRepositoryProtocolSuite) TestARefusedExtensionWritesNothing() {
	refusal := model.Refusal{Code: "probe.refused", Path: "provides;rest", Source: "api.yaml", Details: map[string]any{"graph": "petstore_graph"}}
	store := newRecordingExtensionStore()
	store.refusal, store.refuses = refusal, true
	uploaded := this.uploadedPetsContract("pets_refused")
	uploaded.Extensions = map[string]any{"rest": "petstore_graph"}

	result := this.repositoryWithExtensions(store).Publish(
		context.Background(),
		uploaded,
		func(*model.PersistedContract) model.ContractDiff { return model.ContractDiff{} },
		func([]model.ContractFile) bool { return true },
	)

	this.Equal(repository.PublishResult{Outcome: repository.PublishRefused, Refusal: refusal}, result)
	this.Equal(0, this.countContractsOf(uploaded.ParticipantID))
	this.Empty(store.inserted)
}

func (this *ContractRepositoryProtocolSuite) TestRefuseContractExtensionsAsksTheStoreOfEachCarriedExtension() {
	refusal := model.Refusal{Code: "probe.refused", Path: "provides;rest", Source: "api.yaml", Details: map[string]any{}}
	store := newRecordingExtensionStore()
	store.refusal, store.refuses = refusal, true
	repo := this.repositoryWithExtensions(store)

	_, refusedWithout := repo.RefuseContractExtensions(context.Background(), &model.PersistedContract{})
	got, refusedWith := repo.RefuseContractExtensions(context.Background(), &model.PersistedContract{Extensions: map[string]any{"rest": "petstore_graph"}})

	this.False(refusedWithout)
	this.True(refusedWith)
	this.Equal(refusal, got)
	this.Equal([]any{"petstore_graph"}, store.refusalAsked)
}
