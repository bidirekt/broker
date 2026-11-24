package contractfiles

import (
	"slices"
	"strings"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/protocol"
	"github.com/bidirekt/broker/internal/contracts/validator"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

func HasBlankSource(files []model.ContractFile) bool {
	return slices.ContainsFunc(files, func(file model.ContractFile) bool {
		return strings.TrimSpace(file.Source) == ""
	})
}

type Interpreter struct {
	registry *protocol.Registry
}

func NewInterpreter(registry *protocol.Registry) *Interpreter {
	return &Interpreter{registry: registry}
}

func (this *Interpreter) ToUploadedContract(
	files []model.ContractFile,
	participant *model.Participant,
	version string,
) (uploadedContract *model.UploadedContract, violations []violation.Violation, readErr error, err error) {
	declarations, violations, readErr := this.toCheckedDeclarations(files)
	if readErr != nil || len(violations) > 0 {
		return nil, violations, readErr, nil
	}

	resources := fragmentmapper.ToResourceModels(declarations)

	uploadedContract = model.NewUploadedContract(participant.ID, participant.Name, version, files)
	uploadedContract.Schemas = fragmentmapper.ToSchemaModels(declarations)

	for _, resource := range resources {
		if err = uploadedContract.AddResource(&resource); err != nil {
			return nil, nil, nil, err
		}
	}

	return uploadedContract, nil, nil, nil
}

func (this *Interpreter) ToViolations(files []model.ContractFile) ([]violation.Violation, error) {
	_, violations, readErr := this.toCheckedDeclarations(files)

	return violations, readErr
}

func (this *Interpreter) toCheckedDeclarations(files []model.ContractFile) (protocol.Declarations, []violation.Violation, error) {
	fragments, shapeViolations, readErr := this.toFragments(files)
	if readErr != nil || len(shapeViolations) > 0 {
		return protocol.Declarations{}, shapeViolations, readErr
	}

	declarations := fragmentmapper.ToDeclarations(fragments, this.registry)

	return declarations, validator.Validate(declarations), nil
}

func (this *Interpreter) toFragments(files []model.ContractFile) ([]contract.Fragment, []violation.Violation, error) {
	fragments := make([]contract.Fragment, 0, len(files))
	for _, file := range files {
		fragment, err := contract.DecodeFragment(file.Source, file.Content)
		if err != nil {
			return nil, nil, err
		}

		fragments = append(fragments, fragment)
	}

	var shapeViolations []violation.Violation
	for _, fragment := range contract.SortedBySource(fragments) {
		shapeViolations = append(shapeViolations, descriptor.Validate(this.registry.Grammar(), fragment.Document, fragment.Source)...)
	}

	return fragments, shapeViolations, nil
}
