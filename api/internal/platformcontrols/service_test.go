package platformcontrols

import (
	"context"
	"errors"
	"testing"
)

type fakeStore struct {
	control    Control
	controlErr error
	controls   []Control
	setResult  TransitionResult
	setErr     error
}

func (s *fakeStore) ControlForPlatform(context.Context, string) (Control, error) {
	return s.control, s.controlErr
}

func (s *fakeStore) List(context.Context) ([]Control, error) {
	return s.controls, nil
}

func (s *fakeStore) SetState(context.Context, TransitionRequest) (TransitionResult, error) {
	return s.setResult, s.setErr
}

func TestServiceEvaluateStateMatrix(t *testing.T) {
	tests := []struct {
		name    string
		state   State
		op      Operation
		blocked bool
	}{
		{name: "active connect", state: StateActive, op: OperationConnect},
		{name: "active publish", state: StateActive, op: OperationPublish},
		{name: "active read", state: StateActive, op: OperationRead},
		{name: "publishing disabled connect", state: StatePublishingDisabled, op: OperationConnect},
		{name: "publishing disabled publish", state: StatePublishingDisabled, op: OperationPublish, blocked: true},
		{name: "publishing disabled read", state: StatePublishingDisabled, op: OperationRead},
		{name: "fully disabled connect", state: StateFullyDisabled, op: OperationConnect, blocked: true},
		{name: "fully disabled publish", state: StateFullyDisabled, op: OperationPublish, blocked: true},
		{name: "fully disabled read", state: StateFullyDisabled, op: OperationRead, blocked: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(&fakeStore{control: Control{
				Platform:    "tiktok",
				State:       tt.state,
				ReasonCode:  "security_incident",
				UserMessage: "TikTok is temporarily unavailable.",
				Version:     3,
			}})
			decision, err := service.Evaluate(context.Background(), " TikTok ", tt.op)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Blocked != tt.blocked || decision.Platform != "tiktok" || decision.State != tt.state || decision.Operation != tt.op {
				t.Fatalf("decision=%+v, blocked=%v", decision, tt.blocked)
			}
			if decision.Version != 3 || decision.ReasonCode != "security_incident" {
				t.Fatalf("decision metadata=%+v", decision)
			}
		})
	}
}

func TestServiceEvaluateRejectsUnknownPlatformBeforeStoreRead(t *testing.T) {
	_, err := NewService(&fakeStore{}).Evaluate(context.Background(), "mastodon", OperationPublish)
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("err=%v, want ErrUnsupportedPlatform", err)
	}
}

func TestServiceEvaluateRejectsUnknownOperation(t *testing.T) {
	_, err := NewService(&fakeStore{}).Evaluate(context.Background(), "tiktok", Operation("delete_everything"))
	if !errors.Is(err, ErrUnsupportedOperation) {
		t.Fatalf("err=%v, want ErrUnsupportedOperation", err)
	}
}

func TestServiceEvaluatePropagatesStoreFailure(t *testing.T) {
	want := errors.New("database unavailable")
	_, err := NewService(&fakeStore{controlErr: want}).Evaluate(context.Background(), "tiktok", OperationRead)
	if !errors.Is(err, want) {
		t.Fatalf("err=%v, want %v", err, want)
	}
}

func TestServiceSetStateValidatesAndDelegates(t *testing.T) {
	want := TransitionResult{Control: Control{Platform: "tiktok", State: StateFullyDisabled, Version: 2}, Changed: true}
	store := &fakeStore{setResult: want}
	got, err := NewService(store).SetState(context.Background(), TransitionRequest{
		Platform: " TikTok ", State: StateFullyDisabled, ExpectedVersion: 1, ActorUserID: "user_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Changed || got.Control.Platform != "tiktok" || got.Control.Version != 2 {
		t.Fatalf("result=%+v", got)
	}
}

func TestServiceSetStateRejectsInvalidState(t *testing.T) {
	_, err := NewService(&fakeStore{}).SetState(context.Background(), TransitionRequest{
		Platform: "tiktok", State: State("broken"), ExpectedVersion: 1,
	})
	if !errors.Is(err, ErrUnsupportedState) {
		t.Fatalf("err=%v, want ErrUnsupportedState", err)
	}
}
