package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
		_, prior, requestErr := existing.RequestResult(cmd.RequestID, "create_case", hashCreateCase(cmd), actor.Name)
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
	if err := c.RecordRequest(cmd.RequestID, "create_case", hashCreateCase(cmd), actor.Name, now); err != nil {
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
		}, hashReplacePlan(cmd))
}

func (s *Service) Evaluate(ctx context.Context, caseID string, actor Actor, cmd EvaluateCommand) (CaseView, error) {
	if err := validateActor(actor, domain.RoleTechnicalDirector); err != nil {
		return CaseView{}, err
	}
	c, prior, err := s.prepareChange(ctx, caseID, "evaluate", cmd.RequestID, cmd.ExpectedRevision, hashEvaluate(cmd), actor.Name)
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
	if err := c.RecordRequest(cmd.RequestID, "evaluate", hashEvaluate(cmd), actor.Name, now); err != nil {
		return CaseView{}, err
	}
	if err := s.repo.Save(ctx, c, cmd.ExpectedRevision, c.DrainEvents()); err != nil {
		return CaseView{}, err
	}
	return toCaseView(c), nil
}

func (s *Service) SubmitEvidence(ctx context.Context, caseID string, actor Actor, cmd SubmitEvidenceCommand) (CaseView, error) {
	if err := validateActor(actor, domain.RoleMechanicalLead); err != nil {
		return CaseView{}, err
	}
	c, prior, err := s.prepareChange(ctx, caseID, "submit_evidence", cmd.RequestID, cmd.ExpectedRevision, hashSubmitEvidence(cmd), actor.Name)
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
	if err := c.RecordRequest(cmd.RequestID, "submit_evidence", hashSubmitEvidence(cmd), actor.Name, now); err != nil {
		return CaseView{}, err
	}
	if err := s.repo.Save(ctx, c, cmd.ExpectedRevision, c.DrainEvents()); err != nil {
		return CaseView{}, err
	}
	return toCaseView(c), nil
}

func (s *Service) RequestReview(ctx context.Context, caseID string, actor Actor, cmd RequestReviewCommand) (CaseView, error) {
	return s.change(ctx, caseID, actor, "request_review", cmd.RequestID, cmd.ExpectedRevision,
		[]domain.Role{domain.RoleMechanicalLead}, func(c *domain.ClearanceCase, now time.Time) error {
			return c.RequestReview(actor.Name, now)
		}, hashRequestReview(cmd))
}

func (s *Service) ReviewFinding(ctx context.Context, caseID string, actor Actor, cmd ReviewFindingCommand) (CaseView, error) {
	return s.change(ctx, caseID, actor, "review_finding", cmd.RequestID, cmd.ExpectedRevision,
		[]domain.Role{domain.RoleSafetyReviewer}, func(c *domain.ClearanceCase, now time.Time) error {
			return c.ReviewFinding(cmd.FindingID, cmd.Accepted, cmd.Note, actor.Name, now)
		}, hashReviewFinding(cmd))
}

func (s *Service) Sign(ctx context.Context, caseID string, actor Actor, cmd SignCommand) (CertificateView, error) {
	if err := validateActor(actor, domain.RoleSafetyReviewer); err != nil {
		return CertificateView{}, err
	}
	c, prior, err := s.prepareChange(ctx, caseID, "sign", cmd.RequestID, cmd.ExpectedRevision, hashSign(cmd), actor.Name)
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
	if err := c.RecordRequest(cmd.RequestID, "sign", hashSign(cmd), actor.Name, now); err != nil {
		return CertificateView{}, err
	}
	if err := s.repo.Save(ctx, c, cmd.ExpectedRevision, c.DrainEvents()); err != nil {
		return CertificateView{}, err
	}
	return CertificateView{Certificate: cert, Valid: true}, nil
}

func (s *Service) change(ctx context.Context, caseID string, actor Actor, command, requestID string, expectedRevision int64, roles []domain.Role, mutate func(*domain.ClearanceCase, time.Time) error, requestHash string) (CaseView, error) {
	if err := validateActor(actor, roles...); err != nil {
		return CaseView{}, err
	}
	c, prior, err := s.prepareChange(ctx, caseID, command, requestID, expectedRevision, requestHash, actor.Name)
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
	if err := c.RecordRequest(requestID, command, requestHash, actor.Name, now); err != nil {
		return CaseView{}, err
	}
	if err := s.repo.Save(ctx, c, expectedRevision, c.DrainEvents()); err != nil {
		return CaseView{}, err
	}
	return toCaseView(c), nil
}

func (s *Service) prepareChange(ctx context.Context, caseID, command, requestID string, expectedRevision int64, requestHash, actor string) (*domain.ClearanceCase, bool, error) {
	if strings.TrimSpace(requestID) == "" {
		return nil, false, domain.NewValidation("request_id", "不能为空")
	}
	c, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return nil, false, err
	}
	_, prior, err := c.RequestResult(requestID, command, requestHash, actor)
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

func hashCreateCase(cmd CreateCaseCommand) string {
	return contentHash(struct {
		PerformanceName string    `json:"performance_name"`
		VenueZone       string    `json:"venue_zone"`
		StartsAt        time.Time `json:"starts_at"`
		EndsAt          time.Time `json:"ends_at"`
	}{PerformanceName: cmd.PerformanceName, VenueZone: cmd.VenueZone, StartsAt: cmd.StartsAt, EndsAt: cmd.EndsAt})
}

func hashReplacePlan(cmd ReplacePlanCommand) string {
	return contentHash(struct {
		Steps []domain.MotionStep `json:"steps"`
	}{Steps: cmd.Steps})
}

func hashEvaluate(cmd EvaluateCommand) string {
	return contentHash(struct{}{})
}

func hashSubmitEvidence(cmd SubmitEvidenceCommand) string {
	return contentHash(struct {
		FindingID      string `json:"finding_id"`
		OriginalName   string `json:"original_name"`
		MediaType      string `json:"media_type"`
		ExpectedSHA256 string `json:"sha256"`
		Note           string `json:"note"`
		Content        []byte `json:"content"`
	}{FindingID: cmd.FindingID, OriginalName: cmd.OriginalName, MediaType: cmd.MediaType, ExpectedSHA256: cmd.ExpectedSHA256, Note: cmd.Note, Content: cmd.Content})
}

func hashRequestReview(cmd RequestReviewCommand) string {
	return contentHash(struct{}{})
}

func hashReviewFinding(cmd ReviewFindingCommand) string {
	return contentHash(struct {
		FindingID string `json:"finding_id"`
		Accepted  bool   `json:"accepted"`
		Note      string `json:"note"`
	}{FindingID: cmd.FindingID, Accepted: cmd.Accepted, Note: cmd.Note})
}

func hashSign(cmd SignCommand) string {
	return contentHash(struct{}{})
}

func contentHash(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
