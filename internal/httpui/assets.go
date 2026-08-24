package httpui

import (
	"embed"
	"io"
	"net/http"
	"path"
	"strings"
)

//go:embed web/*
var embeddedWeb embed.FS

func (h *Handler) Root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		h.writeError(w, http.StatusNotFound, "not_found", "页面不存在", nil)
		return
	}
	http.Redirect(w, r, "/workbench", http.StatusTemporaryRedirect)
}

func (h *Handler) Workbench(w http.ResponseWriter, _ *http.Request) {
	h.serveEmbedded(w, "web/workbench.html", "text/html; charset=utf-8")
}

func (h *Handler) Asset(w http.ResponseWriter, r *http.Request) {
	name := path.Base(r.PathValue("name"))
	if name != r.PathValue("name") || strings.Contains(name, "..") {
		h.writeError(w, http.StatusBadRequest, "invalid_path", "资源路径无效", nil)
		return
	}
	contentType := "application/octet-stream"
	switch path.Ext(name) {
	case ".css":
		contentType = "text/css; charset=utf-8"
	case ".js":
		contentType = "text/javascript; charset=utf-8"
	}
	h.serveEmbedded(w, "web/"+name, contentType)
}

func (h *Handler) serveEmbedded(w http.ResponseWriter, name, contentType string) {
	file, err := h.assets.Open(name)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "资源不存在", nil)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.Copy(w, file)
}
