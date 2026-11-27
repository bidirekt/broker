package repository

import (
	"context"

	"github.com/bidirekt/broker/internal/model"
)

// ContractExtensionStore keeps what a protocol adds to a whole contract, its value in the contract's Extensions
// under the protocol's name; every method panics on a database error.
type ContractExtensionStore interface {
	Protocol() model.Protocol
	RefuseContractExtension(ctx context.Context, q Querier, extension any) (model.Refusal, bool)
	InsertContractExtension(ctx context.Context, q Querier, contractID int64, extension any)
	LoadContractExtensions(ctx context.Context, q Querier, contractIDs []int64) map[int64]any
}

func (this *ContractRepository) refuseExtensions(ctx context.Context, q Querier, extensions map[string]any) (model.Refusal, bool) {
	for _, store := range this.extensions {
		extension, carried := extensions[store.Protocol().String()]
		if !carried {
			continue
		}

		if refusal, refused := store.RefuseContractExtension(ctx, q, extension); refused {
			return refusal, true
		}
	}

	return model.Refusal{}, false
}

func (this *ContractRepository) insertExtensions(ctx context.Context, q Querier, contract *model.UploadedContract) {
	for _, store := range this.extensions {
		if extension, carried := contract.Extensions[store.Protocol().String()]; carried {
			store.InsertContractExtension(ctx, q, contract.ID, extension)
		}
	}
}

func (this *ContractRepository) loadExtensions(ctx context.Context, q Querier, contractID int64) map[string]any {
	var extensions map[string]any

	for _, store := range this.extensions {
		extension, found := store.LoadContractExtensions(ctx, q, []int64{contractID})[contractID]
		if !found {
			continue
		}

		if extensions == nil {
			extensions = map[string]any{}
		}

		extensions[store.Protocol().String()] = extension
	}

	return extensions
}

// RefuseContractExtensions answers, outside any transaction, whether a publish of the contract would be refused.
func (this *ContractRepository) RefuseContractExtensions(ctx context.Context, contract *model.PersistedContract) (model.Refusal, bool) {
	return this.refuseExtensions(ctx, this.pool, contract.Extensions)
}
