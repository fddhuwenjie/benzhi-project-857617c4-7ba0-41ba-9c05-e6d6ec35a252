package regression

import (
	"context"
	"testing"
	"time"

	"stage-clearance/internal/application"
	"stage-clearance/internal/domain"
	"stage-clearance/internal/rules"
	"stage-clearance/internal/store"
)

func TestRequestIDMustBindRequestPayload(t *testing.T) {
	repo, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	service := application.NewService(repo, repo, rules.NewDefaultEngine(), application.FixedClock{Value: now}, nil)
	actor := application.Actor{Name: "技术总监", Role: domain.RoleTechnicalDirector}
	first := application.CreateCaseCommand{
		RequestID:       "same-request",
		PerformanceName: "首版演出",
		VenueZone:       "main",
		StartsAt:        now.Add(time.Hour),
		EndsAt:          now.Add(2 * time.Hour),
	}
	if _, err := service.CreateCase(context.Background(), actor, first); err != nil {
		t.Fatal(err)
	}

	changed := first
	changed.PerformanceName = "被替换的演出"
	if _, err := service.CreateCase(context.Background(), actor, changed); err == nil {
		t.Fatalf("TestRequestIDMustBindRequestPayload: 相同 request_id 携带不同请求内容时被错误地当作幂等重放")
	}
}
