package repository

import (
	"context"
	"fmt"

	"github.com/bidirekt/broker/internal/model"
)

// ProtocolTable reads and writes a protocol's own resource table; every method panics on a database error.
type ProtocolTable interface {
	Protocol() model.Protocol
	Insert(ctx context.Context, q Querier, resourceID int64, identity model.ResourceIdentity)
	LoadIdentities(ctx context.Context, q Querier, resourceIDs []int64) map[int64]model.ResourceIdentity
	LoadProviderHint(ctx context.Context, q Querier, consumer model.PersistedResource, environmentID int64) (hint any, found bool)
}

func toProtocolTables(tables []ProtocolTable) map[model.Protocol]ProtocolTable {
	byProtocol := make(map[model.Protocol]ProtocolTable, len(tables))
	for _, table := range tables {
		byProtocol[table.Protocol()] = table
	}

	return byProtocol
}

func (this *ContractRepository) tableOf(resourceID int64, protocol model.Protocol) ProtocolTable {
	table, registered := this.tables[protocol]
	if !registered {
		panic(fmt.Errorf("resource %d has protocol %q, which has no registered table", resourceID, protocol))
	}

	return table
}

func (this *ContractRepository) identitiesOf(ctx context.Context, q Querier, rows []tableRow) map[int64]model.ResourceIdentity {
	idsByProtocol := make(map[model.Protocol][]int64)
	for _, row := range rows {
		protocol := model.Protocol(row.ResourceProtocol)
		idsByProtocol[protocol] = append(idsByProtocol[protocol], row.ResourceID)
	}

	identities := make(map[int64]model.ResourceIdentity, len(rows))
	for protocol, ids := range idsByProtocol {
		for id, identity := range this.tableOf(ids[0], protocol).LoadIdentities(ctx, q, ids) {
			identities[id] = identity
		}
	}

	for _, row := range rows {
		if _, found := identities[row.ResourceID]; !found {
			panic(fmt.Errorf("resource %d has no row in its protocol table", row.ResourceID))
		}
	}

	return identities
}
