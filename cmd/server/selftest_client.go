package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"stage-clearance/internal/application"
	"stage-clearance/internal/domain"
)

type smokeClient struct {
	baseURL string
	client  *http.Client
	serial  int
}

type wireEnvelope[T any] struct {
	Data  T `json:"data"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func newSmokeClient(address string) *smokeClient {
	return &smokeClient{
		baseURL: "http://" + address,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *smokeClient) requestID(action string) string {
	c.serial++
	return fmt.Sprintf("selftest-%s-%d", action, c.serial)
}

func (c *smokeClient) waitReady(ctx context.Context) error {
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/ready", nil)
		if err != nil {
			return err
		}
		response, err := c.client.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待服务就绪: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (c *smokeClient) json(ctx context.Context, method, path string, actor application.Actor, body any, target any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if actor.Name != "" {
		request.Header.Set("X-Actor-Name", actor.Name)
		request.Header.Set("X-Actor-Role", string(actor.Role))
	}
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var failure wireEnvelope[json.RawMessage]
		_ = json.Unmarshal(payload, &failure)
		return fmt.Errorf("%s %s 返回 %d: %s", method, path, response.StatusCode, failure.Error.Message)
	}
	if target == nil {
		return nil
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return err
	}
	return json.Unmarshal(envelope.Data, target)
}

func (c *smokeClient) evidence(ctx context.Context, caseID, findingID string, revision int64, actor application.Actor) (application.CaseView, error) {
	content := []byte("舞台机械整改证据\n风险项: " + findingID + "\n复核状态: 已完成现场确认\n")
	digest := sha256.Sum256(content)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"request_id":        c.requestID("evidence"),
		"expected_revision": strconv.FormatInt(revision, 10),
		"note":              "现场复测完成，互锁与净空由机械主管双人确认。",
		"sha256":            hex.EncodeToString(digest[:]),
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return application.CaseView{}, err
		}
	}
	part, err := writer.CreateFormFile("file", "整改复测记录.txt")
	if err != nil {
		return application.CaseView{}, err
	}
	if _, err := part.Write(content); err != nil {
		return application.CaseView{}, err
	}
	if err := writer.Close(); err != nil {
		return application.CaseView{}, err
	}
	path := fmt.Sprintf("/api/cases/%s/findings/%s/evidence", url.PathEscape(caseID), url.PathEscape(findingID))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, &body)
	if err != nil {
		return application.CaseView{}, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("X-Actor-Name", actor.Name)
	request.Header.Set("X-Actor-Role", string(actor.Role))
	response, err := c.client.Do(request)
	if err != nil {
		return application.CaseView{}, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return application.CaseView{}, err
	}
	if response.StatusCode != http.StatusOK {
		return application.CaseView{}, fmt.Errorf("提交证据返回 %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	var envelope wireEnvelope[application.CaseView]
	if err := json.Unmarshal(data, &envelope); err != nil {
		return application.CaseView{}, err
	}
	return envelope.Data, nil
}

func smokeSteps() []domain.MotionStep {
	return []domain.MotionStep{
		{ID: "cue-001", Sequence: 1, DeviceCode: "HOIST-A", Zone: "main", StartsAtOffsetMS: 1000, DurationMS: 9000, LoadKG: 640, RequiresClearance: true, ClearanceConfirmed: false, InterlockCodes: []string{"E-STOP"}},
		{ID: "cue-002", Sequence: 2, DeviceCode: "TRACK-1", Zone: "main", StartsAtOffsetMS: 6000, DurationMS: 7000, LoadKG: 220, RequiresClearance: false, ClearanceConfirmed: true, InterlockCodes: []string{"E-STOP", "TRACK-LIMIT"}},
	}
}
