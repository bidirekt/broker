package contract_differ

import "github.com/bidirekt/broker/internal/model"

// DiffContracts reports the resource and property changes from the persisted current to the uploaded next, keyed by resource hash.
func DiffContracts(current *model.PersistedContract, next *model.UploadedContract) model.ContractDiff {
	return DiffResourceProperties(loadedProperties(current), uploadedProperties(next))
}

func loadedProperties(contract *model.PersistedContract) map[string]model.ResourceProperties {
	properties := make(map[string]model.ResourceProperties, len(contract.Resources))
	for key, resource := range contract.Resources {
		properties[key] = resource.Properties
	}

	return properties
}

func uploadedProperties(contract *model.UploadedContract) map[string]model.ResourceProperties {
	out := make(map[string]model.ResourceProperties, len(contract.Resources))
	for key, resource := range contract.Resources {
		out[key] = resource.Properties
	}
	return out
}

func DiffResourceProperties(current, next map[string]model.ResourceProperties) model.ContractDiff {
	resourcesChanges := map[string]model.ResourceChange{}

	for resourceKey, oldProperties := range current {
		nextProperties, exists := next[resourceKey]
		// If the resource is not present in the new contract, it was removed
		if !exists {
			resourcesChanges[resourceKey] = removedResourceChange(oldProperties)
			continue
		}

		// If the resource is present in the new contract and is different, it was modified
		if resourceChanges, resourceWasChanged := modifiedResourceChange(oldProperties, nextProperties); resourceWasChanged {
			resourcesChanges[resourceKey] = resourceChanges
		}
	}

	for newResourceKey, newProperties := range next {
		// If the resource is not present in the previous contract, it was added
		if _, exists := current[newResourceKey]; !exists {
			resourcesChanges[newResourceKey] = addedResourceChange(newProperties)
		}
	}

	return model.ContractDiff{
		Resources: resourcesChanges,
	}
}

func addedResourceChange(properties model.ResourceProperties) model.ResourceChange {
	propertiesChanged := make(map[string]model.PropertyChange, len(properties))

	for propertyPath, property := range properties {
		propertiesChanged[propertyPath] = model.PropertyChange{Kind: model.ChangeAdded, After: property}
	}

	return model.ResourceChange{Kind: model.ChangeAdded, Properties: propertiesChanged}
}

func removedResourceChange(properties model.ResourceProperties) model.ResourceChange {
	propertiesChanged := make(map[string]model.PropertyChange, len(properties))

	for propertyPath, property := range properties {
		propertiesChanged[propertyPath] = model.PropertyChange{Kind: model.ChangeRemoved, Before: property}
	}

	return model.ResourceChange{Kind: model.ChangeRemoved, Properties: propertiesChanged}
}

func modifiedResourceChange(prev, next model.ResourceProperties) (model.ResourceChange, bool) {
	propertiesChanged := map[string]model.PropertyChange{}

	for propertyPath, property := range prev {
		nextProperty, exists := next[propertyPath]
		// If the property is not present in the next resource, it was removed
		if !exists {
			propertiesChanged[propertyPath] = model.PropertyChange{Kind: model.ChangeRemoved, Before: property}
			continue
		}

		// If the property is present in the next resource and is different, it was modified
		if !property.IsSame(&nextProperty) {
			propertiesChanged[propertyPath] = model.PropertyChange{Kind: model.ChangeModified, Before: property, After: nextProperty}
		}
	}

	for path, nextProperty := range next {
		// If the property is not present in the previous resource, it was added
		if _, exists := prev[path]; !exists {
			propertiesChanged[path] = model.PropertyChange{Kind: model.ChangeAdded, After: nextProperty}
		}
	}

	// If no properties were changed, the resource was not modified
	if len(propertiesChanged) == 0 {
		return model.ResourceChange{}, false
	}

	// If properties were changed, the resource was modified
	return model.ResourceChange{
		Kind:       model.ChangeModified,
		Properties: propertiesChanged,
	}, true
}
