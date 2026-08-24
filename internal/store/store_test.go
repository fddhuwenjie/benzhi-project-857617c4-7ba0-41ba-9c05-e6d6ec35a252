package store

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
)

func newStoredCase(t *testing.T, s *FileStore) *domain.ClearanceCase {
	t.Helper()
	now := time.Now().UTC()
	c, err := domain.NewCase(domain.NewCaseInput{ID: "case-store", ClearanceNumber: "SC-STORE", PerformanceName: "存储测试", VenueZone: "main", StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour), CreatedBy: "测试员", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Create(context.Background(), c, c.DrainEvents()); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestFileStoreRevisionConflict(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c := newStoredCase(t, s)
	stale, err := s.Get(context.Background(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	current, err := s.Get(context.Background(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	step := domain.MotionStep{ID: "step-1", Sequence: 1, DeviceCode: "HOIST-A", Zone: "main", DurationMS: 1000}
	if err := current.ReplacePlan([]domain.MotionStep{step}, "测试员", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(context.Background(), current, 1, current.DrainEvents()); err != nil {
		t.Fatal(err)
	}
	if err := stale.ReplacePlan([]domain.MotionStep{step}, "测试员", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(context.Background(), stale, 1, stale.DrainEvents()); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("期望 revision 冲突，得到 %v", err)
	}
	events, err := s.Timeline(context.Background(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("冲突提交不应写审计，事件数=%d", len(events))
	}
}

func TestEvidenceDigestAndStartupValidation(t *testing.T) {
	root := t.TempDir()
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("evidence")
	digest := sha256.Sum256(content)
	want := hex.EncodeToString(digest[:])
	key, size, err := s.PutEvidence(context.Background(), want, content)
	if err != nil || size != int64(len(content)) {
		t.Fatalf("保存证据失败: key=%s size=%d err=%v", key, size, err)
	}
	if _, _, err := s.PutEvidence(context.Background(), "0000", content); !errors.Is(err, domain.ErrDigestMismatch) {
		t.Fatalf("错误摘要应被拒绝，得到 %v", err)
	}
	path := filepath.Join(root, "evidence", want[:2], want)
	if err := os.WriteFile(path, []byte("tampered"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadEvidence(context.Background(), key); !errors.Is(err, domain.ErrDigestMismatch) {
		t.Fatalf("附件篡改应被识别，得到 %v", err)
	}
}
