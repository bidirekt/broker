package contract_differ

import "github.com/bidirekt/broker/internal/model"

func DiffContracts(current *model.PersistedContract, next *model.UploadedContract) model.ContractDiff {
	return model.ContractDiff{
		Resources: diffResources(current.Resources, next.Resources),
		Schemas:   diffSchemas(current.Schemas, next.Schemas),
	}
}

func RemovedSinceDeployed(deployed *model.PersistedContract, next *model.UploadedContract) map[string]model.PersistedResource {
	removed := make(map[string]model.PersistedResource)
	for key, kind := range DiffContracts(deployed, next).Resources {
		resource := deployed.Resources[key]
		if kind != model.ChangeRemoved || resource.Removed {
			continue
		}

		resource.Removed = true
		removed[key] = resource
	}

	return removed
}

func diffResources(current map[string]model.PersistedResource, next map[string]model.UploadedResource) map[string]model.ChangeKind {
	changes := map[string]model.ChangeKind{}

	for hash := range current {
		if _, kept := next[hash]; !kept {
			changes[hash] = model.ChangeRemoved
		}
	}

	for hash := range next {
		if _, existed := current[hash]; !existed {
			changes[hash] = model.ChangeAdded
		}
	}

	return changes
}

func diffSchemas(current, next model.Schemas) map[string]model.SchemaChange {
	changes := map[string]model.SchemaChange{}

	for name, schema := range current {
		nextSchema, kept := next[name]
		if !kept {
			changes[name] = model.SchemaChange{Kind: model.ChangeRemoved, Fields: diffFields(schema, nil)}
			continue
		}

		if fields := diffFields(schema, nextSchema); len(fields) > 0 {
			changes[name] = model.SchemaChange{Kind: model.ChangeModified, Fields: fields}
		}
	}

	for name, schema := range next {
		if _, existed := current[name]; !existed {
			changes[name] = model.SchemaChange{Kind: model.ChangeAdded, Fields: diffFields(nil, schema)}
		}
	}

	return changes
}

func diffFields(current, next model.Schema) map[string]model.FieldChange {
	changes := map[string]model.FieldChange{}

	for path, field := range current {
		nextField, kept := next[path]
		if !kept {
			changes[path] = model.FieldChange{Kind: model.ChangeRemoved, Before: field}
			continue
		}

		if field != nextField {
			changes[path] = model.FieldChange{Kind: model.ChangeModified, Before: field, After: nextField}
		}
	}

	for path, field := range next {
		if _, existed := current[path]; !existed {
			changes[path] = model.FieldChange{Kind: model.ChangeAdded, After: field}
		}
	}

	return changes
}
