package httpui

import (
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"stage-clearance/internal/application"
	"stage-clearance/internal/domain"
	"stage-clearance/internal/store"
)

const multipartOverhead = 1 << 20

func (h *Handler) SubmitEvidence(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(h, w, r)
	if !ok {
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		h.writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "证据提交必须使用 multipart/form-data", nil)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, store.MaxEvidenceBytes+multipartOverhead)
	if err := r.ParseMultipartForm(store.MaxEvidenceBytes + multipartOverhead); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_multipart", "附件表单无效或超过大小限制", nil)
		return
	}
	defer r.MultipartForm.RemoveAll()
	revision, err := parseRevision(r.FormValue("expected_revision"))
	if err != nil {
		h.handleServiceError(w, domain.NewValidation("expected_revision", err.Error()))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		h.handleServiceError(w, domain.NewValidation("file", "必须提供本地证据附件"))
		return
	}
	defer file.Close()
	content, err := readMultipartFile(file)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	mediaType := header.Header.Get("Content-Type")
	if mediaType == "" {
		mediaType = http.DetectContentType(content)
	}
	cmd := application.SubmitEvidenceCommand{
		RequestID: r.FormValue("request_id"), ExpectedRevision: revision,
		FindingID: r.PathValue("findingID"), OriginalName: header.Filename,
		MediaType: mediaType, ExpectedSHA256: r.FormValue("sha256"),
		Note: r.FormValue("note"), Content: content,
	}
	view, err := h.service.SubmitEvidence(r.Context(), r.PathValue("caseID"), actor, cmd)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, view)
}

func readMultipartFile(file multipart.File) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(file, store.MaxEvidenceBytes+1))
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, domain.NewValidation("file", "证据附件不能为空")
	}
	if len(content) > store.MaxEvidenceBytes {
		return nil, domain.NewValidation("file", "证据附件超过 10MiB 限制")
	}
	return content, nil
}
