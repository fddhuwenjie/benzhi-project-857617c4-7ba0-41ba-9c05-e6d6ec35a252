package domain

import "context"

type Repository interface {
	Create(ctx context.Context, c *ClearanceCase, events []AuditEvent) error
	Get(ctx context.Context, id string) (*ClearanceCase, error)
	Save(ctx context.Context, c *ClearanceCase, expectedRevision int64, events []AuditEvent) error
	List(ctx context.Context) ([]*ClearanceCase, error)
	Timeline(ctx context.Context, caseID string) ([]AuditEvent, error)
	FindCertificate(ctx context.Context, clearanceNumber, verificationCode string) (*ReleaseCertificate, error)
}

type EvidenceBlobStore interface {
	PutEvidence(ctx context.Context, expectedSHA256 string, content []byte) (storageKey string, size int64, err error)
	ReadEvidence(ctx context.Context, storageKey string) ([]byte, error)
	DeleteEvidenceIfUnreferenced(ctx context.Context, storageKey string) error
}

type FindingSpec struct {
	Key          string
	MotionStepID string
	RuleCode     string
	Severity     Severity
	Message      string
	Location     string
}

type RuleEvaluator interface {
	Version() string
	Evaluate(ctx context.Context, c *ClearanceCase) ([]FindingSpec, error)
}
