package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"stage-clearance/internal/domain"
)

type Service struct {
	repo      domain.Repository
	blobs     domain.EvidenceBlobStore
	evaluator domain.RuleEvaluator
	clock     Clock
	ids       IDGenerator
}

func NewService(repo domain.Repository, blobs domain.EvidenceBlobStore, evaluator domain.RuleEvaluator, clock Clock, ids IDGenerator) *Service {
	if clock == nil {
		clock = SystemClock{}
	}
	if ids == nil {
		ids = RandomIDGenerator{}
	}
	return &Service{repo: repo, blobs: blobs, evaluator: evaluator, clock: clock, ids: ids}
}

func (s *Service) CreateCase(ctx context.Context, actor Actor, cmd CreateCaseCommand) (CaseView, error) {
	if err := validateActor(actor, domain.RoleTechnicalDirector); err != nil {
		return CaseView{}, err
	}
	if strings.TrimSpace(cmd.RequestID) == "" {
		return CaseView{}, domain.NewValidation("request_id", "不能为空")
	}
	caseID := stableCaseID(actor.Name, cmd.RequestID)
	if existing, err := s.repo.Get(ctx, caseID); err == nil {
		_, prior, requestErr := existing.RequestResult(cmd.RequestID, "create_case")
		if requestErr != nil {
			return CaseView{}, requestErr
		}
		if prior {
			return toCaseView(existing), nil
		}
		return CaseView{}, domain.ErrConflict
	} else if !errors.Is(err, domain.ErrNotFound) {
		return CaseView{}, err
	}
	now := s.clock.Now()
	clearanceNumber := fmt.Sprintf("SC-%s-%s", now.Format("20060102"), shortID(caseID))
	c, err := domain.NewCase(domain.NewCaseInput{
		ID: caseID, ClearanceNumber: clearanceNumber,
		PerformanceName: cmd.PerformanceName, VenueZone: cmd.VenueZone,
		StartsAt: cmd.StartsAt, EndsAt: cmd.EndsAt,
		CreatedBy: actor.Name, Now: now,
	})
	if err != nil {
		return CaseView{}, err
	}
	if err := c.RecordRequest(cmd.RequestID, "create_case", now); err != nil {
		return CaseView{}, err
	}
	if err := s.repo.Create(ctx, c, c.DrainEvents()); err != nil {
		return CaseView{}, err
	}
	return toCaseView(c), nil
}

func (s *Service) ReplacePlan(ctx context.Context, caseID string, actor Actor, cmd ReplacePlanCommand) (CaseView, error) {
	return s.change(ctx, caseID, actor, "replace_plan", cmd.RequestID, cmd.ExpectedRevision,
		[]domain.Role{domain.RoleTechnicalDirector}, func(c *domain.ClearanceCase, now time.Time) error {
			return c.ReplacePlan(cmd.Steps, actor.Name, now)
		})
}

func (s *Service) Evaluate(ctx context.Context, caseID string, actor Actor, cmd EvaluateCommand) (CaseView, error) {
	if err := validateActor(actor, domain.RoleTechnicalDirector); err != nil {
		return CaseView{}, err
	}
	c, prior, err := s.prepareChange(ctx, caseID, "evaluate", cmd.RequestID, cmd.ExpectedRevision)
	if err != nil {
		return CaseView{}, err
	}
	if prior {
		return toCaseView(c), nil
	}
	now := s.clock.Now()
	if err := c.SubmitEvaluation(actor.Name, now); err != nil {
		return CaseView{}, err
	}
	specs, err := s.evaluator.Evaluate(ctx, c)
	if err != nil {
		return CaseView{}, err
	}
	findings := make([]domain.RiskFinding, 0, len(specs))
	for _, spec := range specs {
		findings = append(findings, domain.RiskFinding{
			ID:           stableFindingID(c.ID, s.evaluator.Version(), spec.Key),
			MotionStepID: spec.MotionStepID, RuleCode: spec.RuleCode,
			Severity: spec.Severity, Message: spec.Message, Location: spec.Location,
		})
	}
	if err := c.ApplyEvaluation(findings, s.evaluator.Version(), actor.Name, now); err != nil {
		return CaseView{}, err
	}
	if err := c.RecordRequest(cmd.RequestID, "evaluate", now); err != nil {
		return CaseView{}, err
	}
	stored, err := s.saveChange(ctx, c, cmd.ExpectedRevision, caseID, cmd.RequestID, "evaluate")
	if err != nil {
		return CaseView{}, err
	}
	return toCaseView(stored), nil
}

func (s *Service) SubmitEvidence(ctx context.Context, caseID string, actor Actor, cmd SubmitEvidenceCommand) (CaseView, error) {
	if err := validateActor(actor, domain.RoleMechanicalLead); err != nil {
		return CaseView{}, err
	}
	c, prior, err := s.prepareChange(ctx, caseID, "submit_evidence", cmd.RequestID, cmd.ExpectedRevision)
	if err != nil {
		return CaseView{}, err
	}
	if prior {
		return toCaseView(c), nil
	}
	storageKey, size, err := s.blobs.PutEvidence(ctx, cmd.ExpectedSHA256, cmd.Content)
	if err != nil {
		return CaseView{}, err
	}
	digest := sha256.Sum256(cmd.Content)
	evidence := domain.EvidenceRecord{
		ID: s.ids.New("evidence"), OriginalName: strings.TrimSpace(cmd.OriginalName),
		MediaType: strings.TrimSpace(cmd.MediaType), SizeBytes: size,
		SHA256: hex.EncodeToString(digest[:]), StorageKey: storageKey, Note: cmd.Note,
	}
	now := s.clock.Now()
	if err := c.AddEvidence(cmd.FindingID, evidence, actor.Name, now); err != nil {
		return CaseView{}, err
	}
	if err := c.RecordRequest(cmd.RequestID, "submit_evidence", now); err != nil {
		return CaseView{}, err
	}
	stored, err := s.saveChange(ctx, c, cmd.ExpectedRevision, caseID, cmd.RequestID, "submit_evidence")
	if err != nil {
		return CaseView{}, err
	}
	return toCaseView(stored), nil
}

func (s *Service) RequestReview(ctx context.Context, caseID string, actor Actor, cmd RequestReviewCommand) (CaseView, error) {
	return s.change(ctx, caseID, actor, "request_review", cmd.RequestID, cmd.ExpectedRevision,
		[]domain.Role{domain.RoleMechanicalLead}, func(c *domain.ClearanceCase, now time.Time) error {
			return c.RequestReview(actor.Name, now)
		})
}

func (s *Service) ReviewFinding(ctx context.Context, caseID string, actor Actor, cmd ReviewFindingCommand) (CaseView, error) {
	return s.change(ctx, caseID, actor, "review_finding", cmd.RequestID, cmd.ExpectedRevision,
		[]domain.Role{domain.RoleSafetyReviewer}, func(c *domain.ClearanceCase, now time.Time) error {
			return c.ReviewFinding(cmd.FindingID, cmd.Accepted, cmd.Note, actor.Name, now)
		})
}

func (s *Service) Sign(ctx context.Context, caseID string, actor Actor, cmd SignCommand) (CertificateView, error) {
	if err := validateActor(actor, domain.RoleSafetyReviewer); err != nil {
		return CertificateView{}, err
	}
	c, prior, err := s.prepareChange(ctx, caseID, "sign", cmd.RequestID, cmd.ExpectedRevision)
	if err != nil {
		return CertificateView{}, err
	}
	if prior {
		if c.Certificate == nil {
			return CertificateView{}, domain.ErrInvalidState
		}
		return CertificateView{Certificate: *c.Certificate, Valid: domain.VerifyCertificate(*c.Certificate)}, nil
	}
	now := s.clock.Now()
	cert, err := domain.NewCertificate(s.ids.New("certificate"), c, actor.Name, now)
	if err != nil {
		return CertificateView{}, err
	}
	if err := c.MarkReleased(cert, actor.Name, now); err != nil {
		return CertificateView{}, err
	}
	if err := c.RecordRequest(cmd.RequestID, "sign", now); err != nil {
		return CertificateView{}, err
	}
	stored, err := s.saveChange(ctx, c, cmd.ExpectedRevision, caseID, cmd.RequestID, "sign")
	if err != nil {
		return CertificateView{}, err
	}
	if stored.Certificate == nil {
		return CertificateView{}, domain.ErrInvalidState
	}
	return CertificateView{Certificate: *stored.Certificate, Valid: domain.VerifyCertificate(*stored.Certificate)}, nil
}

func (s *Service) change(ctx context.Context, caseID string, actor Actor, command, requestID string, expectedRevision int64, roles []domain.Role, mutate func(*domain.ClearanceCase, time.Time) error) (CaseView, error) {
	if err := validateActor(actor, roles...); err != nil {
		return CaseView{}, err
	}
	c, prior, err := s.prepareChange(ctx, caseID, command, requestID, expectedRevision)
	if err != nil {
		return CaseView{}, err
	}
	if prior {
		return toCaseView(c), nil
	}
	now := s.clock.Now()
	if err := mutate(c, now); err != nil {
		return CaseView{}, err
	}
	if err := c.RecordRequest(requestID, command, now); err != nil {
		return CaseView{}, err
	}
	stored, err := s.saveChange(ctx, c, expectedRevision, caseID, requestID, command)
	if err != nil {
		return CaseView{}, err
	}
	return toCaseView(stored), nil
}

// saveChange persists the mutated case and resolves concurrent idempotent
// replays. When two identical requests race on the same unprocessed snapshot,
// the first Save succeeds and the second observes ErrConflict because the
// revision advanced. In that case the stored case already records the request,
// so this returns the persisted view instead of surfacing the conflict.
// Genuine conflicts from different request identifiers still surface as
// ErrConflict.
func (s *Service) saveChange(ctx context.Context, c *domain.ClearanceCase, expectedRevision int64, caseID, requestID, command string) (*domain.ClearanceCase, error) {
	if err := s.repo.Save(ctx, c, expectedRevision, c.DrainEvents()); err != nil {
		if !errors.Is(err, domain.ErrConflict) {
			return nil, err
		}
		stored, getErr := s.repo.Get(ctx, caseID)
		if getErr != nil {
			return nil, err
		}
		if _, prior, reqErr := stored.RequestResult(requestID, command); reqErr == nil && prior {
			return stored, nil
		}
		return nil, err
	}
	return c, nil
}

func (s *Service) prepareChange(ctx context.Context, caseID, command, requestID string, expectedRevision int64) (*domain.ClearanceCase, bool, error) {
	if strings.TrimSpace(requestID) == "" {
		return nil, false, domain.NewValidation("request_id", "不能为空")
	}
	c, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return nil, false, err
	}
	_, prior, err := c.RequestResult(requestID, command)
	if err != nil {
		return nil, false, err
	}
	if prior {
		return c, true, nil
	}
	if expectedRevision <= 0 || c.Revision != expectedRevision {
		return nil, false, domain.ErrConflict
	}
	return c, false, nil
}

func validateActor(actor Actor, roles ...domain.Role) error {
	if strings.TrimSpace(actor.Name) == "" {
		return domain.NewValidation("actor.name", "不能为空")
	}
	return domain.ValidateRole(actor.Role, roles...)
}

func shortID(id string) string {
	clean := strings.ReplaceAll(id, "-", "")
	if len(clean) > 8 {
		return strings.ToUpper(clean[len(clean)-8:])
	}
	return strings.ToUpper(clean)
}

func stableFindingID(caseID, version, key string) string {
	digest := sha256.Sum256([]byte(caseID + "\n" + version + "\n" + key))
	return "finding-" + hex.EncodeToString(digest[:])[:20]
}

func stableCaseID(actor, requestID string) string {
	digest := sha256.Sum256([]byte(actor + "\n" + requestID))
	return "case-" + hex.EncodeToString(digest[:])[:20]
}
