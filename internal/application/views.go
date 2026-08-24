package application

import (
	"sort"
	"time"

	"stage-clearance/internal/domain"
)

type CaseView struct {
	ID              string                     `json:"id"`
	ClearanceNumber string                     `json:"clearance_number"`
	PerformanceName string                     `json:"performance_name"`
	VenueZone       string                     `json:"venue_zone"`
	StartsAt        time.Time                  `json:"starts_at"`
	EndsAt          time.Time                  `json:"ends_at"`
	Status          domain.CaseStatus          `json:"status"`
	Revision        int64                      `json:"revision"`
	RuleVersion     string                     `json:"rule_version,omitempty"`
	CreatedBy       string                     `json:"created_by"`
	CreatedAt       time.Time                  `json:"created_at"`
	UpdatedAt       time.Time                  `json:"updated_at"`
	Steps           []domain.MotionStep        `json:"steps"`
	Findings        []domain.RiskFinding       `json:"findings"`
	Certificate     *domain.ReleaseCertificate `json:"certificate,omitempty"`
	Progress        ProgressView               `json:"progress"`
}

type ProgressView struct {
	TotalFindings     int `json:"total_findings"`
	EvidenceSubmitted int `json:"evidence_submitted"`
	Accepted          int `json:"accepted"`
	Returned          int `json:"returned"`
}

type QueueItem struct {
	CaseID          string            `json:"case_id"`
	ClearanceNumber string            `json:"clearance_number"`
	PerformanceName string            `json:"performance_name"`
	VenueZone       string            `json:"venue_zone"`
	StartsAt        time.Time         `json:"starts_at"`
	Revision        int64             `json:"revision"`
	Status          domain.CaseStatus `json:"status"`
	FindingCount    int               `json:"finding_count"`
	AcceptedCount   int               `json:"accepted_count"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type TimelineView struct {
	CaseID string              `json:"case_id"`
	Events []domain.AuditEvent `json:"events"`
}

type CertificateView struct {
	Certificate domain.ReleaseCertificate `json:"certificate"`
	Valid       bool                      `json:"valid"`
}

func toCaseView(c *domain.ClearanceCase) CaseView {
	steps := append([]domain.MotionStep(nil), c.Steps...)
	findings := append([]domain.RiskFinding(nil), c.Findings...)
	view := CaseView{
		ID: c.ID, ClearanceNumber: c.ClearanceNumber,
		PerformanceName: c.PerformanceName, VenueZone: c.VenueZone,
		StartsAt: c.StartsAt, EndsAt: c.EndsAt, Status: c.Status,
		Revision: c.Revision, RuleVersion: c.RuleVersion,
		CreatedBy: c.CreatedBy, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		Steps: steps, Findings: findings,
		Progress: ProgressView{TotalFindings: len(findings)},
	}
	if c.Certificate != nil {
		cert := *c.Certificate
		view.Certificate = &cert
	}
	for _, finding := range findings {
		if finding.Evidence != nil {
			view.Progress.EvidenceSubmitted++
		}
		switch finding.Status {
		case domain.FindingAccepted:
			view.Progress.Accepted++
		case domain.FindingReturned:
			view.Progress.Returned++
		}
	}
	return view
}

func toQueueItem(c *domain.ClearanceCase) QueueItem {
	item := QueueItem{
		CaseID: c.ID, ClearanceNumber: c.ClearanceNumber,
		PerformanceName: c.PerformanceName, VenueZone: c.VenueZone,
		StartsAt: c.StartsAt, Revision: c.Revision, Status: c.Status,
		FindingCount: len(c.Findings), UpdatedAt: c.UpdatedAt,
	}
	for _, finding := range c.Findings {
		if finding.Status == domain.FindingAccepted {
			item.AcceptedCount++
		}
	}
	return item
}

func sortQueue(items []QueueItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].StartsAt.Equal(items[j].StartsAt) {
			return items[i].CaseID < items[j].CaseID
		}
		return items[i].StartsAt.Before(items[j].StartsAt)
	})
}
