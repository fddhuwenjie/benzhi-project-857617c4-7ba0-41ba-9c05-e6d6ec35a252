package restart_evidence_integrity_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"stage-clearance/internal/domain"
	"stage-clearance/internal/store"
)

func TestRestartMustRejectCorruptedReferencedEvidence(t *testing.T) {
	root := t.TempDir()
	s, err := store.New(root)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	c, err := domain.NewCase(domain.NewCaseInput{
		ID: "case-restart-integrity", ClearanceNumber: "SC-INTEGRITY",
		PerformanceName: "重启完整性测试", VenueZone: "main",
		StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour),
		CreatedBy: "技术总监", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Create(context.Background(), c, c.DrainEvents()); err != nil {
		t.Fatal(err)
	}

	step := domain.MotionStep{
		ID: "step-1", Sequence: 1, DeviceCode: "HOIST-A", Zone: "main",
		DurationMS: 5000, LoadKG: 700, InterlockCodes: []string{"E-STOP"},
	}
	if err := c.ReplacePlan([]domain.MotionStep{step}, "技术总监", now); err != nil {
		t.Fatal(err)
	}
	if err := c.SubmitEvaluation("技术总监", now); err != nil {
		t.Fatal(err)
	}
	finding := domain.RiskFinding{
		ID: "finding-1", MotionStepID: step.ID, RuleCode: "LOAD_LIMIT",
		Severity: domain.SeverityCritical, Message: "超载", Location: "动作 step-1",
	}
	if err := c.ApplyEvaluation([]domain.RiskFinding{finding}, "SC-RULES-2026.1", "技术总监", now); err != nil {
		t.Fatal(err)
	}

	content := []byte("valid referenced evidence")
	digest := sha256.Sum256(content)
	sha := hex.EncodeToString(digest[:])
	key, size, err := s.PutEvidence(context.Background(), sha, content)
	if err != nil {
		t.Fatal(err)
	}
	evidence := domain.EvidenceRecord{
		ID: "evidence-1", OriginalName: "proof.txt", MediaType: "text/plain",
		SizeBytes: size, SHA256: sha, StorageKey: key, Note: "整改完成",
	}
	if err := c.AddEvidence(finding.ID, evidence, "机械主管", now); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(context.Background(), c, 1, c.DrainEvents()); err != nil {
		t.Fatal(err)
	}

	evidencePath := filepath.Join(root, "evidence", sha[:2], sha)
	if err := os.WriteFile(evidencePath, []byte("corrupted after persistence"), 0o640); err != nil {
		t.Fatal(err)
	}

	reopened, err := store.New(root)
	if err != nil {
		return
	}
	_, readErr := reopened.ReadEvidence(context.Background(), key)
	if !errors.Is(readErr, domain.ErrDigestMismatch) {
		t.Fatalf("损坏附件应在读取时暴露摘要错误，得到 %v", readErr)
	}
	t.Fatalf("重启未拒绝损坏的证据附件，后续读取才返回 %v", readErr)
}
