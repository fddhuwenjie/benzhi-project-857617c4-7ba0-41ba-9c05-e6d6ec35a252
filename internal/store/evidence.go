package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"stage-clearance/internal/domain"
)

const MaxEvidenceBytes = 10 << 20

func (s *FileStore) PutEvidence(ctx context.Context, expectedSHA256 string, content []byte) (string, int64, error) {
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}
	if len(content) == 0 {
		return "", 0, domain.NewValidation("content", "证据文件不能为空")
	}
	if len(content) > MaxEvidenceBytes {
		return "", 0, domain.NewValidation("content", "证据文件超过 10MiB 限制")
	}
	digest := sha256.Sum256(content)
	actual := hex.EncodeToString(digest[:])
	if expectedSHA256 != "" && !strings.EqualFold(expectedSHA256, actual) {
		return "", 0, domain.ErrDigestMismatch
	}
	storageKey := actual[:2] + "/" + actual
	dir := filepath.Join(s.evidenceDir, actual[:2])
	path := filepath.Join(dir, actual)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", 0, err
	}
	if existing, err := os.ReadFile(path); err == nil {
		existingDigest := sha256.Sum256(existing)
		if hex.EncodeToString(existingDigest[:]) != actual {
			return "", 0, domain.ErrDigestMismatch
		}
		return storageKey, int64(len(existing)), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", 0, err
	}
	if err := atomicWrite(path, content, 0o640); err != nil {
		return "", 0, fmt.Errorf("保存证据附件: %w", err)
	}
	return storageKey, int64(len(content)), nil
}

func (s *FileStore) ReadEvidence(ctx context.Context, storageKey string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parts := strings.Split(storageKey, "/")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 64 || parts[0] != parts[1][:2] || !isHex(parts[1]) {
		return nil, domain.NewValidation("storage_key", "格式不安全")
	}
	path := filepath.Join(s.evidenceDir, parts[0], parts[1])
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != parts[1] {
		return nil, domain.ErrDigestMismatch
	}
	return data, nil
}

func isHex(value string) bool {
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func (s *FileStore) validateEvidenceRecord(record domain.EvidenceRecord) error {
	parts := strings.Split(record.StorageKey, "/")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 64 || parts[0] != parts[1][:2] || !isHex(parts[1]) {
		return domain.NewValidation("storage_key", "格式不安全")
	}
	path := filepath.Join(s.evidenceDir, parts[0], parts[1])
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("证据附件 %s 不可读: %w: %w", record.StorageKey, domain.ErrDigestMismatch, err)
	}
	digest := sha256.Sum256(data)
	actual := hex.EncodeToString(digest[:])
	if actual != record.SHA256 || actual != parts[1] || int64(len(data)) != record.SizeBytes {
		return fmt.Errorf("证据附件 %s 完整性校验失败: %w", record.StorageKey, domain.ErrDigestMismatch)
	}
	return nil
}
