package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type ClearanceCase struct {
	ID                string                       `json:"id"`
	ClearanceNumber   string                       `json:"clearance_number"`
	PerformanceName   string                       `json:"performance_name"`
	VenueZone         string                       `json:"venue_zone"`
	StartsAt          time.Time                    `json:"starts_at"`
	EndsAt            time.Time                    `json:"ends_at"`
	Status            CaseStatus                   `json:"status"`
	Revision          int64                        `json:"revision"`
	RuleVersion       string                       `json:"rule_version,omitempty"`
	CreatedBy         string                       `json:"created_by"`
	CreatedAt         time.Time                    `json:"created_at"`
	UpdatedAt         time.Time                    `json:"updated_at"`
	Steps             []MotionStep                 `json:"steps"`
	Findings          []RiskFinding                `json:"findings"`
	Certificate       *ReleaseCertificate          `json:"certificate,omitempty"`
	ProcessedRequests map[string]IdempotencyRecord `json:"processed_requests,omitempty"`
	pendingEvents     []AuditEvent
}

type NewCaseInput struct {
	ID              string
	ClearanceNumber string
	PerformanceName string
	VenueZone       string
	StartsAt        time.Time
	EndsAt          time.Time
	CreatedBy       string
	Now             time.Time
}

func NewCase(in NewCaseInput) (*ClearanceCase, error) {
	fields := make([]FieldError, 0)
	if strings.TrimSpace(in.ID) == "" {
		fields = append(fields, FieldError{Field: "id", Message: "不能为空"})
	}
	if strings.TrimSpace(in.PerformanceName) == "" {
		fields = append(fields, FieldError{Field: "performance_name", Message: "不能为空"})
	}
	if strings.TrimSpace(in.VenueZone) == "" {
		fields = append(fields, FieldError{Field: "venue_zone", Message: "不能为空"})
	}
	if in.StartsAt.IsZero() || in.EndsAt.IsZero() || !in.EndsAt.After(in.StartsAt) {
		fields = append(fields, FieldError{Field: "ends_at", Message: "必须晚于有效开始时间"})
	}
	if strings.TrimSpace(in.CreatedBy) == "" {
		fields = append(fields, FieldError{Field: "created_by", Message: "不能为空"})
	}
	if err := CombineValidation(fields...); err != nil {
		return nil, err
	}
	now := in.Now.UTC()
	c := &ClearanceCase{
		ID: in.ID, ClearanceNumber: in.ClearanceNumber,
		PerformanceName: strings.TrimSpace(in.PerformanceName), VenueZone: strings.TrimSpace(in.VenueZone),
		StartsAt: in.StartsAt.UTC(), EndsAt: in.EndsAt.UTC(), Status: StatusDraft,
		Revision: 1, CreatedBy: strings.TrimSpace(in.CreatedBy), CreatedAt: now, UpdatedAt: now,
		Steps: []MotionStep{}, Findings: []RiskFinding{},
		ProcessedRequests: map[string]IdempotencyRecord{},
	}
	c.record("case.created", in.CreatedBy, RoleTechnicalDirector, now, map[string]any{"clearance_number": c.ClearanceNumber})
	return c, nil
}

func (c *ClearanceCase) ReplacePlan(steps []MotionStep, actor string, now time.Time) error {
	if c.Status == StatusReleased {
		return ErrAlreadyReleased
	}
	if c.Status != StatusDraft {
		return ErrInvalidState
	}
	if err := ValidateSteps(c.ID, c.StartsAt, c.EndsAt, steps); err != nil {
		return err
	}
	copySteps := append([]MotionStep(nil), steps...)
	for i := range copySteps {
		copySteps[i].CaseID = c.ID
		copySteps[i].DeviceCode = strings.TrimSpace(copySteps[i].DeviceCode)
		copySteps[i].Zone = strings.TrimSpace(copySteps[i].Zone)
		copySteps[i].InterlockCodes = normalizedStrings(copySteps[i].InterlockCodes)
	}
	sort.Slice(copySteps, func(i, j int) bool {
		if copySteps[i].StartsAtOffsetMS == copySteps[j].StartsAtOffsetMS {
			return copySteps[i].Sequence < copySteps[j].Sequence
		}
		return copySteps[i].StartsAtOffsetMS < copySteps[j].StartsAtOffsetMS
	})
	c.Steps = copySteps
	c.advance(actor, RoleTechnicalDirector, now, "plan.updated", map[string]any{"step_count": len(copySteps)})
	return nil
}

func (c *ClearanceCase) SubmitEvaluation(actor string, now time.Time) error {
	if c.Status != StatusDraft {
		return ErrInvalidState
	}
	if err := ValidateSteps(c.ID, c.StartsAt, c.EndsAt, c.Steps); err != nil {
		return err
	}
	c.Status = StatusPendingEval
	c.advance(actor, RoleTechnicalDirector, now, "evaluation.requested", map[string]any{"step_count": len(c.Steps)})
	return nil
}

func (c *ClearanceCase) ApplyEvaluation(findings []RiskFinding, version, actor string, now time.Time) error {
	if c.Status != StatusPendingEval {
		return ErrInvalidState
	}
	if strings.TrimSpace(version) == "" {
		return NewValidation("rule_version", "不能为空")
	}
	seen := map[string]bool{}
	for i := range findings {
		f := &findings[i]
		if f.ID == "" || f.RuleCode == "" || f.MotionStepID == "" {
			return NewValidation("findings", "规则发现缺少身份或动作定位")
		}
		if seen[f.ID] {
			return NewValidation("findings", "风险项 ID 重复")
		}
		if !c.hasStep(f.MotionStepID) {
			return NewValidation("findings.motion_step_id", "引用了不存在的动作")
		}
		seen[f.ID] = true
		f.CaseID = c.ID
		f.RuleVersion = version
		f.Status = FindingOpen
		f.AssigneeRole = RoleMechanicalLead
	}
	c.Findings = append([]RiskFinding(nil), findings...)
	c.RuleVersion = version
	if len(findings) == 0 {
		c.Status = StatusPendingReview
	} else {
		c.Status = StatusRemediation
	}
	c.advance(actor, RoleTechnicalDirector, now, "evaluation.completed", map[string]any{"finding_count": len(findings), "rule_version": version})
	return nil
}

func (c *ClearanceCase) AddEvidence(findingID string, evidence EvidenceRecord, actor string, now time.Time) error {
	if c.Status != StatusRemediation {
		return ErrInvalidState
	}
	f, err := c.finding(findingID)
	if err != nil {
		return err
	}
	if f.Status == FindingAccepted {
		return NewValidation("finding_id", "已接受风险项不能替换证据")
	}
	if evidence.ID == "" || evidence.SHA256 == "" || evidence.SizeBytes <= 0 || strings.TrimSpace(evidence.Note) == "" {
		return NewValidation("evidence", "证据身份、摘要、大小和整改说明必须完整")
	}
	evidence.FindingID = findingID
	evidence.SubmittedBy = actor
	evidence.SubmittedAt = now.UTC()
	f.Evidence = &evidence
	f.Status = FindingEvidence
	f.ReviewedAt = nil
	f.ReviewedBy = ""
	f.ReviewNote = ""
	c.advance(actor, RoleMechanicalLead, now, "evidence.submitted", map[string]any{"finding_id": findingID, "sha256": evidence.SHA256})
	return nil
}

func (c *ClearanceCase) RequestReview(actor string, now time.Time) error {
	if c.Status != StatusRemediation {
		return ErrInvalidState
	}
	for _, f := range c.Findings {
		if f.Evidence == nil || (f.Status != FindingEvidence && f.Status != FindingAccepted) {
			return fmt.Errorf("%w: %s", ErrEvidenceRequired, f.ID)
		}
	}
	c.Status = StatusPendingReview
	c.advance(actor, RoleMechanicalLead, now, "review.requested", map[string]any{"finding_count": len(c.Findings)})
	return nil
}

func (c *ClearanceCase) ReviewFinding(findingID string, accepted bool, note, reviewer string, now time.Time) error {
	if c.Status != StatusPendingReview {
		return ErrInvalidState
	}
	f, err := c.finding(findingID)
	if err != nil {
		return err
	}
	if f.Evidence == nil {
		return ErrEvidenceRequired
	}
	if f.Status == FindingAccepted && accepted {
		return ErrInvalidState
	}
	if strings.TrimSpace(note) == "" {
		return NewValidation("note", "复核意见不能为空")
	}
	t := now.UTC()
	f.ReviewedBy = strings.TrimSpace(reviewer)
	f.ReviewedAt = &t
	f.ReviewNote = strings.TrimSpace(note)
	eventType := "finding.accepted"
	if accepted {
		f.Status = FindingAccepted
	} else {
		f.Status = FindingReturned
		c.Status = StatusRemediation
		eventType = "finding.returned"
	}
	c.advance(reviewer, RoleSafetyReviewer, now, eventType, map[string]any{"finding_id": findingID, "note": note})
	return nil
}

func (c *ClearanceCase) MarkReleased(cert ReleaseCertificate, reviewer string, now time.Time) error {
	if c.Status != StatusPendingReview {
		return ErrInvalidState
	}
	for _, f := range c.Findings {
		if f.Status != FindingAccepted {
			return ErrReviewIncomplete
		}
	}
	if cert.CaseID != c.ID || cert.CaseRevision != c.Revision+1 || cert.PlanDigest == "" || cert.VerificationCode == "" {
		return NewValidation("certificate", "凭证快照与放行单不一致")
	}
	c.Status = StatusReleased
	c.Certificate = &cert
	c.advance(reviewer, RoleSafetyReviewer, now, "case.released", map[string]any{"certificate_id": cert.ID, "verification_code": cert.VerificationCode})
	return nil
}

func (c *ClearanceCase) RecordRequest(requestID, command, requestHash, actor string, now time.Time) error {
	if strings.TrimSpace(requestID) == "" {
		return NewValidation("request_id", "不能为空")
	}
	if c.ProcessedRequests == nil {
		c.ProcessedRequests = map[string]IdempotencyRecord{}
	}
	if existing, ok := c.ProcessedRequests[requestID]; ok {
		if existing.Command != command {
			return ErrDuplicateRequest
		}
		if existing.RequestHash != "" && existing.RequestHash != requestHash {
			return ErrIdempotencyConflict
		}
		if existing.Actor != "" && actor != "" && existing.Actor != actor {
			return ErrIdempotencyConflict
		}
	}
	c.ProcessedRequests[requestID] = IdempotencyRecord{
		Command: command, Revision: c.Revision, At: now.UTC(),
		RequestHash: requestHash, Actor: actor,
	}
	return nil
}

func (c *ClearanceCase) RequestResult(requestID, command, requestHash, actor string) (IdempotencyRecord, bool, error) {
	r, ok := c.ProcessedRequests[requestID]
	if !ok {
		return IdempotencyRecord{}, false, nil
	}
	if r.Command != command {
		return IdempotencyRecord{}, false, ErrDuplicateRequest
	}
	if r.RequestHash != "" && requestHash != "" && r.RequestHash != requestHash {
		return IdempotencyRecord{}, false, ErrIdempotencyConflict
	}
	if r.Actor != "" && actor != "" && r.Actor != actor {
		return IdempotencyRecord{}, false, ErrIdempotencyConflict
	}
	return r, true, nil
}

func (c *ClearanceCase) DrainEvents() []AuditEvent {
	events := append([]AuditEvent(nil), c.pendingEvents...)
	c.pendingEvents = nil
	return events
}

func (c *ClearanceCase) advance(actor string, role Role, now time.Time, eventType string, details map[string]any) {
	c.Revision++
	c.UpdatedAt = now.UTC()
	c.record(eventType, actor, role, now, details)
}

func (c *ClearanceCase) record(eventType, actor string, role Role, now time.Time, details map[string]any) {
	c.pendingEvents = append(c.pendingEvents, AuditEvent{
		ID:     fmt.Sprintf("%s-%d-%s", c.ID, c.Revision, strings.ReplaceAll(eventType, ".", "-")),
		CaseID: c.ID, Revision: c.Revision, Type: eventType, Actor: actor,
		Role: role, OccurredAt: now.UTC(), Details: details,
	})
}

func (c *ClearanceCase) finding(id string) (*RiskFinding, error) {
	for i := range c.Findings {
		if c.Findings[i].ID == id {
			return &c.Findings[i], nil
		}
	}
	return nil, ErrNotFound
}

func (c *ClearanceCase) hasStep(id string) bool {
	for _, step := range c.Steps {
		if step.ID == id {
			return true
		}
	}
	return false
}

func normalizedStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}
