package contractfiles

import (
	"errors"
	"strings"

	"github.com/bidirekt/broker/internal/contracts/contract"
	"github.com/bidirekt/broker/internal/contracts/descriptor"
	"github.com/bidirekt/broker/internal/contracts/mapper/fragmentmapper"
	"github.com/bidirekt/broker/internal/contracts/validator"
	"github.com/bidirekt/broker/internal/contracts/violation"
	"github.com/bidirekt/broker/internal/model"
)

var ErrBlankSource = errors.New("blank contract file source")

type File struct {
	Source  string `json:"source"`
	Content string `json:"content"`
}

func ToFragments(files []File) ([]contract.Fragment, []violation.Violation, error) {
	fragments := make([]contract.Fragment, 0, len(files))
	for _, file := range files {
		if strings.TrimSpace(file.Source) == "" {
			return nil, nil, ErrBlankSource
		}

		fragment, err := contract.DecodeFragment(file.Source, file.Content)
		if err != nil {
			return nil, nil, err
		}

		fragments = append(fragments, fragment)
	}

	var shapeViolations []violation.Violation
	for _, fragment := range contract.SortedBySource(fragments) {
		shapeViolations = append(shapeViolations, descriptor.Validate(descriptor.Contract, fragment.Document, fragment.Source)...)
	}

	if len(shapeViolations) > 0 {
		return nil, shapeViolations, nil
	}

	return fragments, nil, nil
}

func ToUploadedContract(
	fragments []contract.Fragment,
	participant *model.Participant,
	version string,
	contractContent string,
) (*model.UploadedContract, []violation.Violation, error) {
	declarations := fragmentmapper.ToDeclarations(fragments)

	if violations := validator.Validate(declarations); len(violations) > 0 {
		return nil, violations, nil
	}

	resources := fragmentmapper.ToResourceModels(declarations)

	uploadedContract := model.NewUploadedContract(participant.ID, participant.Name, version, contractContent)
	for _, resource := range resources {
		if err := uploadedContract.AddResource(&resource); err != nil {
			return nil, nil, err
		}
	}

	return uploadedContract, nil, nil
}
