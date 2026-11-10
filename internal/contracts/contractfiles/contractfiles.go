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

func (this *Interpreter) ToFragments(files []model.ContractFile) ([]contract.Fragment, []violation.Violation, error) {
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

	if len(shapeViolations) > 0 {
		return nil, shapeViolations, nil
	}

	return fragments, nil, nil
}

func (this *Interpreter) ToUploadedContract(
	fragments []contract.Fragment,
	participant *model.Participant,
	version string,
	files []model.ContractFile,
) (*model.UploadedContract, []violation.Violation, error) {
	declarations := fragmentmapper.ToDeclarations(fragments, this.registry)

	if violations := validator.Validate(declarations); len(violations) > 0 {
		return nil, violations, nil
	}

	resources := fragmentmapper.ToResourceModels(declarations)

	uploadedContract := model.NewUploadedContract(participant.ID, participant.Name, version, files)
	uploadedContract.Schemas = fragmentmapper.ToSchemaModels(declarations)

	for _, resource := range resources {
		if err := uploadedContract.AddResource(&resource); err != nil {
			return nil, nil, err
		}
	}

	return uploadedContract, nil, nil
}
