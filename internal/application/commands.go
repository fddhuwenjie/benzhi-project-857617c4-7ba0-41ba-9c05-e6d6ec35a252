package application

import (
	"time"

	"stage-clearance/internal/domain"
)

type Actor struct {
	Name string      `json:"name"`
	Role domain.Role `json:"role"`
}

type CreateCaseCommand struct {
	RequestID       string    `json:"request_id"`
	PerformanceName string    `json:"performance_name"`
	VenueZone       string    `json:"venue_zone"`
	StartsAt        time.Time `json:"starts_at"`
	EndsAt          time.Time `json:"ends_at"`
}

type ReplacePlanCommand struct {
	RequestID        string              `json:"request_id"`
	ExpectedRevision int64               `json:"expected_revision"`
	Steps            []domain.MotionStep `json:"steps"`
}

type EvaluateCommand struct {
	RequestID        string `json:"request_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type SubmitEvidenceCommand struct {
	RequestID        string `json:"request_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	FindingID        string `json:"finding_id"`
	OriginalName     string `json:"original_name"`
	MediaType        string `json:"media_type"`
	ExpectedSHA256   string `json:"sha256"`
	Note             string `json:"note"`
	Content          []byte `json:"-"`
}

type RequestReviewCommand struct {
	RequestID        string `json:"request_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type ReviewFindingCommand struct {
	RequestID        string `json:"request_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	FindingID        string `json:"finding_id"`
	Accepted         bool   `json:"accepted"`
	Note             string `json:"note"`
}

type SignCommand struct {
	RequestID        string `json:"request_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type CertificateLookup struct {
	ClearanceNumber  string `json:"clearance_number"`
	VerificationCode string `json:"verification_code"`
}
