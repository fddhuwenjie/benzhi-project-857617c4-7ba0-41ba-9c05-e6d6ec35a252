package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
)

type planDocument struct {
	CaseID          string       `json:"case_id"`
	PerformanceName string       `json:"performance_name"`
	VenueZone       string       `json:"venue_zone"`
	StartsAt        string       `json:"starts_at"`
	EndsAt          string       `json:"ends_at"`
	RuleVersion     string       `json:"rule_version"`
	Steps           []MotionStep `json:"steps"`
}

func PlanDigest(c *ClearanceCase) (string, error) {
	steps := append([]MotionStep(nil), c.Steps...)
	for i := range steps {
		steps[i].InterlockCodes = normalizedStrings(steps[i].InterlockCodes)
	}
	sort.Slice(steps, func(i, j int) bool {
		if steps[i].Sequence == steps[j].Sequence {
			return steps[i].ID < steps[j].ID
		}
		return steps[i].Sequence < steps[j].Sequence
	})
	doc := planDocument{
		CaseID: c.ID, PerformanceName: c.PerformanceName, VenueZone: c.VenueZone,
		StartsAt:    c.StartsAt.UTC().Format("2006-01-02T15:04:05.000000000Z"),
		EndsAt:      c.EndsAt.UTC().Format("2006-01-02T15:04:05.000000000Z"),
		RuleVersion: c.RuleVersion, Steps: steps,
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func CertificateVerificationCode(cert ReleaseCertificate) string {
	payload := cert.ID + "\n" + cert.CaseID + "\n" + cert.ClearanceNumber + "\n" +
		strconv.FormatInt(cert.CaseRevision, 10) + "\n" + cert.PerformanceName + "\n" + cert.VenueZone + "\n" +
		cert.PerformanceStart.UTC().Format("2006-01-02T15:04:05.000000000Z") + "\n" +
		cert.PerformanceEnd.UTC().Format("2006-01-02T15:04:05.000000000Z") + "\n" +
		cert.PlanDigest + "\n" + cert.RuleVersion + "\n" + cert.SignedBy + "\n" +
		cert.SignedAt.UTC().Format("2006-01-02T15:04:05.000000000Z")
	digest := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(digest[:])[:20]
}

func VerifyCertificate(cert ReleaseCertificate) bool {
	return cert.VerificationCode != "" && cert.VerificationCode == CertificateVerificationCode(cert)
}
