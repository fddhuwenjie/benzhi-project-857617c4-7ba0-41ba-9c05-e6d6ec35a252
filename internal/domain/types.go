package domain

import "time"

type CaseStatus string

const (
	StatusDraft         CaseStatus = "draft"
	StatusPendingEval   CaseStatus = "pending_evaluation"
	StatusRemediation   CaseStatus = "remediation"
	StatusPendingReview CaseStatus = "pending_review"
	StatusReleased      CaseStatus = "released"
)

type Role string

const (
	RoleTechnicalDirector Role = "technical_director"
	RoleMechanicalLead    Role = "mechanical_lead"
	RoleSafetyReviewer    Role = "safety_reviewer"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

type FindingStatus string

const (
	FindingOpen     FindingStatus = "open"
	FindingEvidence FindingStatus = "evidence_submitted"
	FindingAccepted FindingStatus = "accepted"
	FindingReturned FindingStatus = "returned"
)

type MotionStep struct {
	ID                 string   `json:"id"`
	CaseID             string   `json:"case_id"`
	Sequence           int      `json:"sequence"`
	DeviceCode         string   `json:"device_code"`
	Zone               string   `json:"zone"`
	StartsAtOffsetMS   int64    `json:"starts_at_offset_ms"`
	DurationMS         int64    `json:"duration_ms"`
	LoadKG             float64  `json:"load_kg"`
	RequiresClearance  bool     `json:"requires_clearance"`
	ClearanceConfirmed bool     `json:"clearance_confirmed"`
	InterlockCodes     []string `json:"interlock_codes"`
}

func (s MotionStep) EndsAtOffsetMS() int64 {
	return s.StartsAtOffsetMS + s.DurationMS
}

type RiskFinding struct {
	ID           string          `json:"id"`
	CaseID       string          `json:"case_id"`
	MotionStepID string          `json:"motion_step_id"`
	RuleCode     string          `json:"rule_code"`
	RuleVersion  string          `json:"rule_version"`
	Severity     Severity        `json:"severity"`
	Message      string          `json:"message"`
	Location     string          `json:"location"`
	Status       FindingStatus   `json:"status"`
	AssigneeRole Role            `json:"assignee_role"`
	Evidence     *EvidenceRecord `json:"evidence,omitempty"`
	ReviewedBy   string          `json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time      `json:"reviewed_at,omitempty"`
	ReviewNote   string          `json:"review_note,omitempty"`
}

type EvidenceRecord struct {
	ID           string    `json:"id"`
	FindingID    string    `json:"finding_id"`
	OriginalName string    `json:"original_name"`
	MediaType    string    `json:"media_type"`
	SizeBytes    int64     `json:"size_bytes"`
	SHA256       string    `json:"sha256"`
	StorageKey   string    `json:"storage_key"`
	Note         string    `json:"note"`
	SubmittedBy  string    `json:"submitted_by"`
	SubmittedAt  time.Time `json:"submitted_at"`
}

type ReleaseCertificate struct {
	ID               string    `json:"id"`
	CaseID           string    `json:"case_id"`
	ClearanceNumber  string    `json:"clearance_number"`
	CaseRevision     int64     `json:"case_revision"`
	PerformanceName  string    `json:"performance_name"`
	VenueZone        string    `json:"venue_zone"`
	PerformanceStart time.Time `json:"performance_start"`
	PerformanceEnd   time.Time `json:"performance_end"`
	PlanDigest       string    `json:"plan_digest"`
	RuleVersion      string    `json:"rule_version"`
	SignedBy         string    `json:"signed_by"`
	SignedAt         time.Time `json:"signed_at"`
	VerificationCode string    `json:"verification_code"`
}

type AuditEvent struct {
	ID         string         `json:"id"`
	CaseID     string         `json:"case_id"`
	Revision   int64          `json:"revision"`
	Type       string         `json:"type"`
	Actor      string         `json:"actor"`
	Role       Role           `json:"role"`
	OccurredAt time.Time      `json:"occurred_at"`
	Details    map[string]any `json:"details,omitempty"`
}

type IdempotencyRecord struct {
	Command     string    `json:"command"`
	Revision    int64     `json:"revision"`
	At          time.Time `json:"at"`
	RequestHash string    `json:"request_hash,omitempty"`
	Actor       string    `json:"actor,omitempty"`
}
