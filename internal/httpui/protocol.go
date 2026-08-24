package httpui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"stage-clearance/internal/application"
	"stage-clearance/internal/domain"
)

const maxJSONBody = 2 << 20

type responseEnvelope struct {
	Data any `json:"data"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string              `json:"code"`
	Message string              `json:"message"`
	Fields  []domain.FieldError `json:"fields,omitempty"`
}

func (h *Handler) decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if mediaType := r.Header.Get("Content-Type"); !strings.HasPrefix(mediaType, "application/json") {
		h.writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "请求必须使用 application/json", nil)
		return false
	}
	reader := http.MaxBytesReader(w, r.Body, maxJSONBody)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "JSON 请求体无效", nil)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "JSON 请求体只能包含一个对象", nil)
		return false
	}
	return true
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(responseEnvelope{Data: value}); err != nil {
		h.logger.Error("encode response", "error", err)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string, fields []domain.FieldError) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{Error: apiError{Code: code, Message: message, Fields: fields}})
}

func (h *Handler) handleServiceError(w http.ResponseWriter, err error) {
	var validation *domain.ValidationError
	switch {
	case errors.As(err, &validation):
		h.writeError(w, http.StatusUnprocessableEntity, "validation_failed", validation.Error(), validation.Fields)
	case errors.Is(err, domain.ErrValidation):
		h.writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error(), nil)
	case errors.Is(err, domain.ErrNotFound):
		h.writeError(w, http.StatusNotFound, "not_found", "记录不存在", nil)
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrDuplicateRequest):
		h.writeError(w, http.StatusConflict, "revision_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrForbidden):
		h.writeError(w, http.StatusForbidden, "forbidden", err.Error(), nil)
	case errors.Is(err, domain.ErrInvalidState), errors.Is(err, domain.ErrAlreadyReleased),
		errors.Is(err, domain.ErrEvidenceRequired), errors.Is(err, domain.ErrReviewIncomplete):
		h.writeError(w, http.StatusConflict, "invalid_state", err.Error(), nil)
	case errors.Is(err, domain.ErrDigestMismatch):
		h.writeError(w, http.StatusUnprocessableEntity, "digest_mismatch", err.Error(), nil)
	default:
		h.logger.Error("request failed", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "服务暂时无法完成请求", nil)
	}
}

func actorFromRequest(r *http.Request) (application.Actor, error) {
	actor := application.Actor{Name: strings.TrimSpace(r.Header.Get("X-Actor-Name")), Role: domain.Role(strings.TrimSpace(r.Header.Get("X-Actor-Role")))}
	if actor.Name == "" {
		return application.Actor{}, domain.NewValidation("X-Actor-Name", "不能为空")
	}
	switch actor.Role {
	case domain.RoleTechnicalDirector, domain.RoleMechanicalLead, domain.RoleSafetyReviewer:
		return actor, nil
	default:
		return application.Actor{}, domain.NewValidation("X-Actor-Role", "岗位无效")
	}
}

func requireActor(h *Handler, w http.ResponseWriter, r *http.Request) (application.Actor, bool) {
	actor, err := actorFromRequest(r)
	if err != nil {
		h.handleServiceError(w, err)
		return application.Actor{}, false
	}
	return actor, true
}

func parseRevision(value string) (int64, error) {
	revision, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || revision <= 0 {
		return 0, fmt.Errorf("expected_revision 必须为正整数")
	}
	return revision, nil
}
