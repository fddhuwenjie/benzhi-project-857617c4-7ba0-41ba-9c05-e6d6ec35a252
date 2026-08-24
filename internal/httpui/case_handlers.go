package httpui

import (
	"net/http"

	"stage-clearance/internal/application"
)

func (h *Handler) ListCases(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListCases(r.Context())
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, items)
}

func (h *Handler) CreateCase(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(h, w, r)
	if !ok {
		return
	}
	var cmd application.CreateCaseCommand
	if !h.decodeJSON(w, r, &cmd) {
		return
	}
	view, err := h.service.CreateCase(r.Context(), actor, cmd)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	w.Header().Set("Location", "/api/cases/"+view.ID)
	h.writeJSON(w, http.StatusCreated, view)
}

func (h *Handler) GetCase(w http.ResponseWriter, r *http.Request) {
	view, err := h.service.GetCase(r.Context(), r.PathValue("caseID"))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, view)
}

func (h *Handler) ReplacePlan(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(h, w, r)
	if !ok {
		return
	}
	var cmd application.ReplacePlanCommand
	if !h.decodeJSON(w, r, &cmd) {
		return
	}
	view, err := h.service.ReplacePlan(r.Context(), r.PathValue("caseID"), actor, cmd)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, view)
}

func (h *Handler) Evaluate(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(h, w, r)
	if !ok {
		return
	}
	var cmd application.EvaluateCommand
	if !h.decodeJSON(w, r, &cmd) {
		return
	}
	view, err := h.service.Evaluate(r.Context(), r.PathValue("caseID"), actor, cmd)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, view)
}

func (h *Handler) RequestReview(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(h, w, r)
	if !ok {
		return
	}
	var cmd application.RequestReviewCommand
	if !h.decodeJSON(w, r, &cmd) {
		return
	}
	view, err := h.service.RequestReview(r.Context(), r.PathValue("caseID"), actor, cmd)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, view)
}

func (h *Handler) ReviewFinding(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(h, w, r)
	if !ok {
		return
	}
	var cmd application.ReviewFindingCommand
	if !h.decodeJSON(w, r, &cmd) {
		return
	}
	cmd.FindingID = r.PathValue("findingID")
	view, err := h.service.ReviewFinding(r.Context(), r.PathValue("caseID"), actor, cmd)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, view)
}

func (h *Handler) Sign(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(h, w, r)
	if !ok {
		return
	}
	var cmd application.SignCommand
	if !h.decodeJSON(w, r, &cmd) {
		return
	}
	view, err := h.service.Sign(r.Context(), r.PathValue("caseID"), actor, cmd)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, view)
}
