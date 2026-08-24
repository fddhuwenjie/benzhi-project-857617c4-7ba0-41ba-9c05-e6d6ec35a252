package application

import (
	"context"
	"strings"

	"stage-clearance/internal/domain"
)

func (s *Service) GetCase(ctx context.Context, caseID string) (CaseView, error) {
	c, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return CaseView{}, err
	}
	return toCaseView(c), nil
}

func (s *Service) ListCases(ctx context.Context) ([]CaseView, error) {
	cases, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]CaseView, 0, len(cases))
	for _, c := range cases {
		views = append(views, toCaseView(c))
	}
	return views, nil
}

func (s *Service) ReviewQueue(ctx context.Context) ([]QueueItem, error) {
	cases, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]QueueItem, 0)
	for _, c := range cases {
		if c.Status == domain.StatusPendingReview {
			items = append(items, toQueueItem(c))
		}
	}
	sortQueue(items)
	return items, nil
}

func (s *Service) Timeline(ctx context.Context, caseID string) (TimelineView, error) {
	events, err := s.repo.Timeline(ctx, caseID)
	if err != nil {
		return TimelineView{}, err
	}
	return TimelineView{CaseID: caseID, Events: events}, nil
}

func (s *Service) VerifyCertificate(ctx context.Context, lookup CertificateLookup) (CertificateView, error) {
	if strings.TrimSpace(lookup.ClearanceNumber) == "" || strings.TrimSpace(lookup.VerificationCode) == "" {
		return CertificateView{}, domain.NewValidation("certificate", "放行编号和校验码不能为空")
	}
	cert, err := s.repo.FindCertificate(ctx, strings.TrimSpace(lookup.ClearanceNumber), strings.TrimSpace(lookup.VerificationCode))
	if err != nil {
		return CertificateView{}, err
	}
	valid := domain.VerifyCertificate(*cert)
	if !valid {
		return CertificateView{}, domain.ErrDigestMismatch
	}
	return CertificateView{Certificate: *cert, Valid: true}, nil
}
