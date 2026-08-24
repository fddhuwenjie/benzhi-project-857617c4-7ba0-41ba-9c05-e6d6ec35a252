package application

import (
	"context"
	"fmt"
)

type HealthStatus struct {
	Ready       bool   `json:"ready"`
	RuleVersion string `json:"rule_version"`
	CaseCount   int    `json:"case_count"`
}

func (s *Service) Health(ctx context.Context) (HealthStatus, error) {
	if s.repo == nil || s.blobs == nil || s.evaluator == nil {
		return HealthStatus{}, fmt.Errorf("应用依赖未完成装配")
	}
	cases, err := s.repo.List(ctx)
	if err != nil {
		return HealthStatus{}, err
	}
	return HealthStatus{Ready: true, RuleVersion: s.evaluator.Version(), CaseCount: len(cases)}, nil
}
