package evaluation_cache_result_alias_test

import (
	"context"
	"testing"
	"time"

	"stage-clearance/internal/domain"
	"stage-clearance/internal/rules"
)

func TestEvaluationCacheMustIsolateReturnedResults(t *testing.T) {
	now := time.Date(2026, time.August, 24, 10, 0, 0, 0, time.UTC)
	plan := &domain.ClearanceCase{
		ID:       "case-cache-alias",
		StartsAt: now,
		EndsAt:   now.Add(time.Hour),
		Steps: []domain.MotionStep{{
			ID: "step-overload", Sequence: 1, DeviceCode: "HOIST-A", Zone: "main",
			DurationMS: 1000, LoadKG: 700,
			InterlockCodes: []string{"E-STOP", "UPPER-LIMIT"},
		}},
	}
	engine := rules.NewDefaultEngine()

	first, err := engine.Evaluate(context.Background(), plan)
	if err != nil {
		t.Fatalf("首次评估失败: %v", err)
	}
	if len(first) != 1 || first[0].RuleCode != "LOAD_LIMIT" {
		t.Fatalf("首次评估结果异常: %#v", first)
	}

	first[0].RuleCode = "CALLER_TAMPERED"
	first[0].Message = "调用方局部展示文案"

	second, err := engine.Evaluate(context.Background(), plan)
	if err != nil {
		t.Fatalf("缓存复评失败: %v", err)
	}
	if len(second) != 1 || second[0].RuleCode != "LOAD_LIMIT" {
		t.Fatalf("缓存结果被首次调用方污染: %#v", second)
	}
}
