package httpui

import (
	"net/http"

	"stage-clearance/internal/application"
)

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.Health(r.Context())
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, status)
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if !h.ready {
		h.writeError(w, http.StatusServiceUnavailable, "not_ready", "服务尚未就绪", nil)
		return
	}
	h.Health(w, r)
}

func (h *Handler) Timeline(w http.ResponseWriter, r *http.Request) {
	view, err := h.service.Timeline(r.Context(), r.PathValue("caseID"))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, view)
}

func (h *Handler) ReviewQueue(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ReviewQueue(r.Context())
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, items)
}

func (h *Handler) VerifyCertificate(w http.ResponseWriter, r *http.Request) {
	lookup := application.CertificateLookup{
		ClearanceNumber:  r.URL.Query().Get("clearance_number"),
		VerificationCode: r.URL.Query().Get("verification_code"),
	}
	view, err := h.service.VerifyCertificate(r.Context(), lookup)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, view)
}
