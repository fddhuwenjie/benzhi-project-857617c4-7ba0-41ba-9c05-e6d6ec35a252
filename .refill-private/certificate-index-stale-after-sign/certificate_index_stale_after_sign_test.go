package certificateindexstaleaftersign

import (
	"context"
	"errors"
	"testing"
	"time"

	"stage-clearance/internal/domain"
	"stage-clearance/internal/store"
)

func TestCertificateLookupMustObserveLaterSign(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	repo, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, err := domain.NewCase(domain.NewCaseInput{
		ID: "case-certificate-cache", ClearanceNumber: "SC-CACHE-001",
		PerformanceName: "缓存失效复现", VenueZone: "main",
		StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour),
		CreatedBy: "技术总监", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, c, c.DrainEvents()); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindCertificate(ctx, "SC-NOT-SIGNED", "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("预热空凭证索引应返回 ErrNotFound，得到 %v", err)
	}

	step := domain.MotionStep{
		ID: "step-1", Sequence: 1, DeviceCode: "HOIST-A", Zone: "main",
		StartsAtOffsetMS: 0, DurationMS: 1_000, LoadKG: 100,
	}
	if err := c.ReplacePlan([]domain.MotionStep{step}, "技术总监", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := c.SubmitEvaluation("技术总监", now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := c.ApplyEvaluation(nil, "SC-RULES-2026.1", "技术总监", now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	cert, err := domain.NewCertificate("cert-cache-1", c, "安全复核员", now.Add(4*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.MarkReleased(cert, "安全复核员", now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, c, 1, c.DrainEvents()); err != nil {
		t.Fatal(err)
	}

	found, err := repo.FindCertificate(ctx, cert.ClearanceNumber, cert.VerificationCode)
	if err != nil {
		t.Fatalf("签署提交后凭证必须立即可查: %v", err)
	}
	if found.ID != cert.ID || !domain.VerifyCertificate(*found) {
		t.Fatalf("查询返回了错误凭证: %#v", found)
	}
}
