package httpui

import (
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"stage-clearance/internal/application"
)

type Handler struct {
	service *application.Service
	logger  *slog.Logger
	assets  fs.FS
	ready   bool
}

func NewHandler(service *application.Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger, assets: embeddedWeb, ready: true}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /workbench", h.Workbench)
	mux.HandleFunc("GET /", h.Root)
	mux.HandleFunc("GET /assets/{name}", h.Asset)
	mux.HandleFunc("GET /api/health", h.Health)
	mux.HandleFunc("GET /api/ready", h.Ready)
	mux.HandleFunc("GET /api/cases", h.ListCases)
	mux.HandleFunc("POST /api/cases", h.CreateCase)
	mux.HandleFunc("GET /api/cases/{caseID}", h.GetCase)
	mux.HandleFunc("PUT /api/cases/{caseID}/plan", h.ReplacePlan)
	mux.HandleFunc("POST /api/cases/{caseID}/evaluate", h.Evaluate)
	mux.HandleFunc("POST /api/cases/{caseID}/findings/{findingID}/evidence", h.SubmitEvidence)
	mux.HandleFunc("POST /api/cases/{caseID}/review-request", h.RequestReview)
	mux.HandleFunc("POST /api/cases/{caseID}/findings/{findingID}/review", h.ReviewFinding)
	mux.HandleFunc("POST /api/cases/{caseID}/sign", h.Sign)
	mux.HandleFunc("GET /api/cases/{caseID}/timeline", h.Timeline)
	mux.HandleFunc("GET /api/review-queue", h.ReviewQueue)
	mux.HandleFunc("GET /api/certificates/verify", h.VerifyCertificate)
	return h.middleware(mux)
}

func (h *Handler) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		requestID := r.Header.Get("X-Request-ID")
		if requestID != "" {
			w.Header().Set("X-Request-ID", requestID)
		}
		next.ServeHTTP(w, r)
		h.logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}
