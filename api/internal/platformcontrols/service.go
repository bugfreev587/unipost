package platformcontrols

import (
	"context"
	"fmt"
	"strings"
)

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Evaluate(ctx context.Context, platform string, operation Operation) (Decision, error) {
	platform, err := NormalizePlatform(platform)
	if err != nil {
		return Decision{}, err
	}
	if !operation.Valid() {
		return Decision{}, fmt.Errorf("%w: %s", ErrUnsupportedOperation, operation)
	}
	control, err := s.store.ControlForPlatform(ctx, platform)
	if err != nil {
		return Decision{}, err
	}
	if !control.State.Valid() {
		return Decision{}, fmt.Errorf("%w: %s", ErrUnsupportedState, control.State)
	}
	decision := Decision{
		Platform:   platform,
		State:      control.State,
		Operation:  operation,
		ReasonCode: control.ReasonCode,
		Message:    control.UserMessage,
		Version:    control.Version,
	}
	decision.Blocked = control.State == StateFullyDisabled ||
		(control.State == StatePublishingDisabled && operation == OperationPublish)
	return decision, nil
}

func (s *Service) List(ctx context.Context) ([]Control, error) {
	return s.store.List(ctx)
}

func (s *Service) SetState(ctx context.Context, request TransitionRequest) (TransitionResult, error) {
	platform, err := NormalizePlatform(request.Platform)
	if err != nil {
		return TransitionResult{}, err
	}
	if !request.State.Valid() {
		return TransitionResult{}, fmt.Errorf("%w: %s", ErrUnsupportedState, request.State)
	}
	if request.ExpectedVersion < 1 {
		return TransitionResult{}, fmt.Errorf("expected_version must be positive")
	}
	request.Platform = platform
	request.ReasonCode = strings.TrimSpace(request.ReasonCode)
	request.UserMessage = strings.TrimSpace(request.UserMessage)
	return s.store.SetState(ctx, request)
}
