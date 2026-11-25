package contractfiles

import (
	"fmt"
	"path/filepath"
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

const yamlFormat = "yaml"

var yamlExtensions = []string{".yaml", ".yml"}

// FileFormat reads the contract files of its extensions; the error is a read error, answered as the file's own.
// Its declarations list their schemas in Schemas only: the catalog is built when they are joined with the others.
type FileFormat interface {
	Extensions() []string
	ToDeclarations(files []model.ContractFile) (protocol.Declarations, error)
}

func HasBlankSource(files []model.ContractFile) bool {
	return slices.ContainsFunc(files, func(file model.ContractFile) bool {
		return strings.TrimSpace(file.Source) == ""
	})
}

type Interpreter struct {
	registry    *protocol.Registry
	formats     []FileFormat
	formatOf    map[string]FileFormat
	unsupported string
}

func NewInterpreter(registry *protocol.Registry, formats []FileFormat) *Interpreter {
	extensions := slices.Clone(yamlExtensions)
	formatOf := map[string]FileFormat{}

	for _, format := range formats {
		for _, extension := range format.Extensions() {
			if slices.Contains(extensions, extension) {
				panic(fmt.Errorf("contract file extension %q is registered twice", extension))
			}

			extensions = append(extensions, extension)
			formatOf[extension] = format
		}
	}

	return &Interpreter{
		registry:    registry,
		formats:     formats,
		formatOf:    formatOf,
		unsupported: strings.Join(extensions[:len(extensions)-1], ", ") + " or " + extensions[len(extensions)-1],
	}
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
	uploadedContract.Extensions = declarations.Extensions

	for _, resource := range resources {
		if err = uploadedContract.AddResource(&resource); err != nil {
			return nil, nil, nil, err
		}
	}

	return uploadedContract, nil, nil, nil
}

func (this *Interpreter) ToViolations(files []model.ContractFile) (extensions map[string]any, violations []violation.Violation, readErr error) {
	declarations, violations, readErr := this.toCheckedDeclarations(files)

	return declarations.Extensions, violations, readErr
}

func (this *Interpreter) toCheckedDeclarations(files []model.ContractFile) (protocol.Declarations, []violation.Violation, error) {
	fragments, formatFiles, readErr := this.toFragmentsAndFormatFiles(files)
	if readErr != nil {
		return protocol.Declarations{}, nil, readErr
	}

	formatDeclarations := make([]protocol.Declarations, 0, len(this.formats))
	for _, format := range this.formats {
		if len(formatFiles[format]) == 0 {
			continue
		}

		declarations, readErr := format.ToDeclarations(formatFiles[format])
		if readErr != nil {
			return protocol.Declarations{}, nil, readErr
		}

		formatDeclarations = append(formatDeclarations, declarations)
	}

	var shapeViolations []violation.Violation
	for _, fragment := range contract.SortedBySource(fragments) {
		shapeViolations = append(shapeViolations, descriptor.Validate(this.registry.Grammar(), fragment.Document, fragment.Source)...)
	}

	if len(shapeViolations) > 0 {
		return protocol.Declarations{}, shapeViolations, nil
	}

	declarations := fragmentmapper.ToDeclarations(fragments, this.registry)
	if len(fragments) > 0 {
		declarations.Formats = []string{yamlFormat}
	}

	for _, joined := range formatDeclarations {
		declarations = joinDeclarations(declarations, joined)
	}

	extensions, mergeViolations := this.registry.MergeExtensions(declarations.ExtensionPieces)
	declarations.Extensions = extensions

	violations := append(mergeViolations, validator.Validate(declarations, this.registry.Rules()...)...)

	return declarations, violation.SortedByLocation(violations), nil
}

// toFragmentsAndFormatFiles decodes the YAML files and groups the others by format, failing on the first unreadable file in request order.
func (this *Interpreter) toFragmentsAndFormatFiles(files []model.ContractFile) ([]contract.Fragment, map[FileFormat][]model.ContractFile, error) {
	fragments := make([]contract.Fragment, 0, len(files))
	formatFiles := map[FileFormat][]model.ContractFile{}

	for _, file := range files {
		extension := strings.ToLower(filepath.Ext(file.Source))

		if format, registered := this.formatOf[extension]; registered {
			formatFiles[format] = append(formatFiles[format], file)

			continue
		}

		if !slices.Contains(yamlExtensions, extension) {
			return nil, nil, fmt.Errorf("unsupported contract file: %s (expected %s)", file.Source, this.unsupported)
		}

		fragment, err := contract.DecodeFragment(file.Source, file.Content)
		if err != nil {
			return nil, nil, err
		}

		fragments = append(fragments, fragment)
	}

	return fragments, formatFiles, nil
}

func joinDeclarations(declarations, joined protocol.Declarations) protocol.Declarations {
	declarations.Resources = append(declarations.Resources, joined.Resources...)
	declarations.Formats = append(declarations.Formats, joined.Formats...)

	for _, schema := range joined.Schemas {
		declarations.AddSchema(schema)
	}

	for name, pieces := range joined.ExtensionPieces {
		for _, piece := range pieces {
			declarations.AddExtensionPiece(name, piece)
		}
	}

	return declarations
}
