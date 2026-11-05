package rename_participant

import (
	"context"

	"github.com/bidirekt/broker/internal/repository"
	"github.com/bidirekt/broker/internal/validations"
)

type RenameParticipantStatus int

const (
	ParticipantRenamed RenameParticipantStatus = iota
	ParticipantNotFound
	ParticipantAlreadyExists
	ParticipantHasPublishedContracts
	ParticipantNameInvalid
	ParticipantRenameable
)

type RenameParticipantInput struct {
	OldName string
	NewName string
}

type RenameParticipantOutput struct {
	Status RenameParticipantStatus
}

type RenameParticipantUseCase struct {
	participantRepository *repository.ParticipantRepository
}

func NewRenameParticipantUseCase(participantRepository *repository.ParticipantRepository) *RenameParticipantUseCase {
	return &RenameParticipantUseCase{participantRepository: participantRepository}
}

func (this *RenameParticipantUseCase) FindTarget(ctx context.Context, name string) RenameParticipantOutput {
	published, found := this.participantRepository.FindPublishedByName(ctx, name)
	if !found {
		return RenameParticipantOutput{Status: ParticipantNotFound}
	}

	if published {
		return RenameParticipantOutput{Status: ParticipantHasPublishedContracts}
	}

	return RenameParticipantOutput{Status: ParticipantRenameable}
}

func (this *RenameParticipantUseCase) Execute(ctx context.Context, input RenameParticipantInput) RenameParticipantOutput {
	// only the new name is judged: a legacy participant whose name predates the
	// spelling rule must still be renameable into a valid one
	if validations.ParticipantName(input.NewName) != nil {
		return RenameParticipantOutput{Status: ParticipantNameInvalid}
	}

	found, conflict, published := this.participantRepository.Rename(ctx, input.OldName, input.NewName)
	if conflict {
		return RenameParticipantOutput{Status: ParticipantAlreadyExists}
	}

	if !found {
		return RenameParticipantOutput{Status: ParticipantNotFound}
	}

	if published {
		return RenameParticipantOutput{Status: ParticipantHasPublishedContracts}
	}

	return RenameParticipantOutput{Status: ParticipantRenamed}
}
