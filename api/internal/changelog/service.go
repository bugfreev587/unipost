package changelog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Store interface {
	GetCandidate(context.Context, string) (CandidateRecord, error)
	ClaimCandidate(context.Context, string, []CandidateStatus, CandidateStatus, string) (CandidateRecord, error)
	MarkCandidateFailed(context.Context, string, string) error
	SetDispatchMetadata(context.Context, string, string, string) error
}

type ServiceConfig struct {
	DashboardBaseURL string
	GitHubRef        string
	GitHubWorkflow   string
	DryRun           bool
	LinkTTL          time.Duration
}

type Service struct {
	store      Store
	signer     *Signer
	dispatcher Dispatcher
	config     ServiceConfig
	now        func() time.Time
}

func NewService(store Store, signer *Signer, dispatcher Dispatcher, config ServiceConfig) *Service {
	if config.LinkTTL <= 0 {
		config.LinkTTL = 24 * time.Hour
	}
	if strings.TrimSpace(config.GitHubWorkflow) == "" {
		config.GitHubWorkflow = "changelog-publish.yml"
	}
	return &Service{
		store:      store,
		signer:     signer,
		dispatcher: dispatcher,
		config:     config,
		now:        time.Now,
	}
}

func (s *Service) BuildActionLinks(record CandidateRecord) ActionLinks {
	now := time.Now
	if s != nil && s.now != nil {
		now = s.now
	}
	return s.BuildActionLinksWithExpiry(record, now().Add(s.config.LinkTTL))
}

func (s *Service) BuildActionLinksWithExpiry(record CandidateRecord, expires time.Time) ActionLinks {
	return ActionLinks{
		Publish: s.actionURL(record, ActionPublish, expires),
		Save:    s.actionURL(record, ActionSave, expires),
		Discard: s.actionURL(record, ActionDiscard, expires),
	}
}

func (s *Service) actionURL(record CandidateRecord, action Action, expires time.Time) string {
	base := strings.TrimRight(strings.TrimSpace(s.config.DashboardBaseURL), "/")
	if base == "" {
		base = "https://app.unipost.dev"
	}
	signature := s.signer.Sign(record.ID, action, expires, record.SourceHash)
	values := url.Values{}
	values.Set("candidate_id", record.ID)
	values.Set("action", string(action))
	values.Set("expires", timeFormatUnix(expires))
	values.Set("signature", signature)
	return base + "/admin/changelog-actions?" + values.Encode()
}

func (s *Service) HandleAction(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if s == nil || s.store == nil || s.signer == nil {
		return ActionResult{}, errors.New("changelog service is not configured")
	}
	if !ValidAction(req.Action) {
		return ActionResult{}, ErrUnsupportedAction
	}
	record, err := s.store.GetCandidate(ctx, req.CandidateID)
	if err != nil {
		return ActionResult{}, err
	}
	if err := s.signer.Verify(record.ID, req.Action, req.ExpiresUnix, record.SourceHash, req.Signature); err != nil {
		return ActionResult{}, err
	}
	switch req.Action {
	case ActionSave:
		claimed, err := s.store.ClaimCandidate(ctx, record.ID, []CandidateStatus{StatusPending, StatusSaved}, StatusSaved, req.ActorAdminID)
		if err != nil {
			return ActionResult{}, err
		}
		return ActionResult{CandidateID: record.ID, Action: req.Action, Status: claimed.Status, Message: "Saved for later"}, nil
	case ActionDiscard:
		claimed, err := s.store.ClaimCandidate(ctx, record.ID, []CandidateStatus{StatusPending, StatusSaved}, StatusDiscarded, req.ActorAdminID)
		if err != nil {
			return ActionResult{}, err
		}
		return ActionResult{CandidateID: record.ID, Action: req.Action, Status: claimed.Status, Message: "Discarded"}, nil
	case ActionPublish:
		claimed, err := s.store.ClaimCandidate(ctx, record.ID, []CandidateStatus{StatusPending, StatusSaved}, StatusPublishing, req.ActorAdminID)
		if err != nil {
			return ActionResult{}, err
		}
		dispatchReq := DispatchRequest{
			CandidateID:     claimed.ID,
			SourceHash:      claimed.SourceHash,
			ActionRequestID: newActionRequestID(),
			RequestedBy:     req.ActorAdminID,
			DryRun:          s.config.DryRun,
			Ref:             s.config.GitHubRef,
			Workflow:        s.config.GitHubWorkflow,
		}
		if s.dispatcher == nil {
			_ = s.store.MarkCandidateFailed(ctx, claimed.ID, "GitHub dispatcher is not configured")
			return ActionResult{}, errors.New("GitHub dispatcher is not configured")
		}
		dispatch, err := s.dispatcher.Dispatch(ctx, dispatchReq)
		if err != nil {
			_ = s.store.MarkCandidateFailed(ctx, claimed.ID, err.Error())
			return ActionResult{}, err
		}
		_ = s.store.SetDispatchMetadata(ctx, claimed.ID, dispatchReq.ActionRequestID, dispatch.WorkflowURL)
		return ActionResult{CandidateID: claimed.ID, Action: req.Action, Status: claimed.Status, Message: "Publish started", WorkflowRunURL: dispatch.WorkflowURL}, nil
	default:
		return ActionResult{}, ErrUnsupportedAction
	}
}

func (s *Service) VerifyActionLink(ctx context.Context, candidateID string, action Action, expiresUnix int64, signature string) (CandidateRecord, error) {
	if s == nil || s.store == nil || s.signer == nil {
		return CandidateRecord{}, errors.New("changelog service is not configured")
	}
	record, err := s.store.GetCandidate(ctx, candidateID)
	if err != nil {
		return CandidateRecord{}, err
	}
	if err := s.signer.Verify(record.ID, action, expiresUnix, record.SourceHash, signature); err != nil {
		return CandidateRecord{}, err
	}
	return record, nil
}

func timeFormatUnix(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}

func newActionRequestID() string {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "car_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return "car_" + hex.EncodeToString(buf[:])
}
