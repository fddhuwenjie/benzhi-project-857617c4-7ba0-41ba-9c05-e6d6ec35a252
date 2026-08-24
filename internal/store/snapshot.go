package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"stage-clearance/internal/domain"
)

const snapshotSchema = "stage-clearance.snapshot.v1"

type snapshotPayload struct {
	Case  domain.ClearanceCase `json:"case"`
	Audit []domain.AuditEvent  `json:"audit"`
}

type snapshotEnvelope struct {
	Schema  string          `json:"schema"`
	SavedAt time.Time       `json:"saved_at"`
	SHA256  string          `json:"sha256"`
	Payload json.RawMessage `json:"payload"`
}

func writeSnapshot(path string, c *domain.ClearanceCase, audit []domain.AuditEvent) error {
	clone, err := domain.CloneCase(c)
	if err != nil {
		return err
	}
	payloadData, err := json.Marshal(snapshotPayload{Case: *clone, Audit: audit})
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payloadData)
	envelope := snapshotEnvelope{
		Schema: snapshotSchema, SavedAt: time.Now().UTC(),
		SHA256: hex.EncodeToString(digest[:]), Payload: payloadData,
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	return atomicWrite(path, data, 0o640)
}

func readSnapshot(path string) (*snapshotPayload, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var envelope snapshotEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("解析快照封装: %w", err)
	}
	if envelope.Schema != snapshotSchema || len(envelope.Payload) == 0 {
		return nil, fmt.Errorf("不支持的快照格式 %q", envelope.Schema)
	}
	digest := sha256.Sum256(envelope.Payload)
	if hex.EncodeToString(digest[:]) != envelope.SHA256 {
		return nil, domain.ErrDigestMismatch
	}
	var payload snapshotPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return nil, fmt.Errorf("解析快照内容: %w", err)
	}
	if payload.Case.ProcessedRequests == nil {
		payload.Case.ProcessedRequests = map[string]domain.IdempotencyRecord{}
	}
	return &payload, nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) (retErr error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	dirHandle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirHandle.Close()
	return dirHandle.Sync()
}
