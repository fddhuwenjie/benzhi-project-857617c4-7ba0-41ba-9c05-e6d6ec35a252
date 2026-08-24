package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"stage-clearance/internal/domain"
)

type FileStore struct {
	root        string
	casesDir    string
	evidenceDir string
	mu          sync.RWMutex
}

func New(root string) (*FileStore, error) {
	if strings.TrimSpace(root) == "" {
		return nil, domain.NewValidation("data_dir", "不能为空")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	s := &FileStore{root: abs, casesDir: filepath.Join(abs, "cases"), evidenceDir: filepath.Join(abs, "evidence")}
	for _, dir := range []string{s.root, s.casesDir, s.evidenceDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("创建持久化目录: %w", err)
		}
	}
	if err := s.validateExisting(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileStore) Root() string { return s.root }

func (s *FileStore) Create(ctx context.Context, c *domain.ClearanceCase, events []domain.AuditEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.ID == "" {
		return domain.NewValidation("case", "缺少放行单")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.casePath(c.ID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return domain.ErrConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeSnapshot(path, c, events)
}

func (s *FileStore) Get(ctx context.Context, id string) (*domain.ClearanceCase, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, err := s.casePath(id)
	if err != nil {
		return nil, err
	}
	snapshot, err := readSnapshot(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return domain.CloneCase(&snapshot.Case)
}

func (s *FileStore) Save(ctx context.Context, c *domain.ClearanceCase, expectedRevision int64, events []domain.AuditEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.ID == "" {
		return domain.NewValidation("case", "缺少放行单")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.casePath(c.ID)
	if err != nil {
		return err
	}
	current, err := readSnapshot(path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if current.Case.Revision != expectedRevision {
		return domain.ErrConflict
	}
	if c.Revision <= expectedRevision {
		return fmt.Errorf("%w: 新 revision 必须递增", domain.ErrConflict)
	}
	allEvents := append(append([]domain.AuditEvent(nil), current.Audit...), events...)
	return writeSnapshot(path, c, allEvents)
}

func (s *FileStore) List(ctx context.Context) ([]*domain.ClearanceCase, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.casesDir)
	if err != nil {
		return nil, err
	}
	items := make([]*domain.ClearanceCase, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		snapshot, err := readSnapshot(filepath.Join(s.casesDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		clone, err := domain.CloneCase(&snapshot.Case)
		if err != nil {
			return nil, err
		}
		items = append(items, clone)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items, nil
}

func (s *FileStore) Timeline(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	path, err := s.casePath(caseID)
	if err != nil {
		return nil, err
	}
	snapshot, err := readSnapshot(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return append([]domain.AuditEvent(nil), snapshot.Audit...), nil
}

func (s *FileStore) FindCertificate(ctx context.Context, clearanceNumber, verificationCode string) (*domain.ReleaseCertificate, error) {
	items, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		cert := item.Certificate
		if cert != nil && cert.ClearanceNumber == clearanceNumber && cert.VerificationCode == verificationCode {
			if !domain.VerifyCertificate(*cert) {
				return nil, fmt.Errorf("%w: 凭证校验码无效", domain.ErrDigestMismatch)
			}
			return domain.CloneCertificate(cert)
		}
	}
	return nil, domain.ErrNotFound
}

func (s *FileStore) validateExisting() error {
	entries, err := os.ReadDir(s.casesDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		snapshot, err := readSnapshot(filepath.Join(s.casesDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("校验快照 %s: %w", entry.Name(), err)
		}
		if snapshot.Case.ID == "" || !domain.ValidStatus(snapshot.Case.Status) {
			return fmt.Errorf("校验快照 %s: 放行单内容无效", entry.Name())
		}
		if snapshot.Case.Certificate != nil && !domain.VerifyCertificate(*snapshot.Case.Certificate) {
			return fmt.Errorf("校验快照 %s: %w", entry.Name(), domain.ErrDigestMismatch)
		}
		if snapshot.Case.Certificate != nil {
			planDigest, err := domain.PlanDigest(&snapshot.Case)
			if err != nil || planDigest != snapshot.Case.Certificate.PlanDigest || snapshot.Case.Status != domain.StatusReleased {
				return fmt.Errorf("校验快照 %s: 凭证方案摘要不一致", entry.Name())
			}
		}
	}
	return nil
}

func (s *FileStore) casePath(id string) (string, error) {
	if !validKey(id) {
		return "", domain.NewValidation("id", "格式不安全")
	}
	return filepath.Join(s.casesDir, id+".json"), nil
}

func validKey(value string) bool {
	if value == "" || len(value) > 128 || value == "." || value == ".." {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

func (s *FileStore) MarshalStatus() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.casesDir)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"root": s.root, "case_files": len(entries)})
}
