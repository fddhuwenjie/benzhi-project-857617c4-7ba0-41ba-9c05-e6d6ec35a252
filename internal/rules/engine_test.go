package rules

import (
	"context"
	"reflect"
	"testing"
	"time"

	"stage-clearance/internal/domain"
)

func TestEvaluateDeterministicAndComplete(t *testing.T) {
	now := time.Now().UTC()
	c := &domain.ClearanceCase{
		ID: "case-1", StartsAt: now, EndsAt: now.Add(time.Hour),
		Steps: []domain.MotionStep{
			{ID: "a", Sequence: 1, DeviceCode: "HOIST-A", Zone: "main", StartsAtOffsetMS: 0, DurationMS: 9000, LoadKG: 700, RequiresClearance: true, InterlockCodes: []string{"E-STOP"}},
			{ID: "b", Sequence: 2, DeviceCode: "TRACK-1", Zone: "main", StartsAtOffsetMS: 5000, DurationMS: 5000, LoadKG: 100, ClearanceConfirmed: true, InterlockCodes: []string{"E-STOP", "TRACK-LIMIT"}},
		},
	}
	engine := NewDefaultEngine()
	first, err := engine.Evaluate(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Evaluate(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("相同方案的规则结果不稳定")
	}
	if len(first) != 4 {
		t.Fatalf("发现数=%d，期望 4", len(first))
	}
	if first[0].Severity != domain.SeverityCritical || first[1].Severity != domain.SeverityCritical {
		t.Fatalf("严重度排序错误: %#v", first)
	}
}

func TestEvaluateRejectsUnknownDevice(t *testing.T) {
	now := time.Now().UTC()
	c := &domain.ClearanceCase{ID: "case-1", StartsAt: now, EndsAt: now.Add(time.Hour), Steps: []domain.MotionStep{{ID: "a", Sequence: 1, DeviceCode: "UNKNOWN", Zone: "main", DurationMS: 1000}}}
	if _, err := NewDefaultEngine().Evaluate(context.Background(), c); err == nil {
		t.Fatal("未知设备不应被视为安全")
	}
}
