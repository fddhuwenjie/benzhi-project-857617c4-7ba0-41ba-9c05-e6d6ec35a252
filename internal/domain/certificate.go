package domain

import (
	"strings"
	"time"
)

func NewCertificate(id string, c *ClearanceCase, signer string, now time.Time) (ReleaseCertificate, error) {
	if c.Status != StatusPendingReview {
		return ReleaseCertificate{}, ErrInvalidState
	}
	for _, finding := range c.Findings {
		if finding.Status != FindingAccepted {
			return ReleaseCertificate{}, ErrReviewIncomplete
		}
	}
	if strings.TrimSpace(id) == "" || strings.TrimSpace(signer) == "" {
		return ReleaseCertificate{}, NewValidation("certificate", "凭证 ID 和签署人不能为空")
	}
	digest, err := PlanDigest(c)
	if err != nil {
		return ReleaseCertificate{}, err
	}
	cert := ReleaseCertificate{
		ID: id, CaseID: c.ID, ClearanceNumber: c.ClearanceNumber,
		CaseRevision: c.Revision + 1, PerformanceName: c.PerformanceName,
		VenueZone: c.VenueZone, PerformanceStart: c.StartsAt, PerformanceEnd: c.EndsAt,
		PlanDigest: digest, RuleVersion: c.RuleVersion,
		SignedBy: strings.TrimSpace(signer), SignedAt: now.UTC(),
	}
	cert.VerificationCode = CertificateVerificationCode(cert)
	return cert, nil
}
