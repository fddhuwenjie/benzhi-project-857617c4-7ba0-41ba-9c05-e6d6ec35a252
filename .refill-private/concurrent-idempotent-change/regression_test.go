package regression

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"stage-clearance/internal/application"
	"stage-clearance/internal/domain"
	"stage-clearance/internal/rules"
	"stage-clearance/internal/store"
)

type synchronizedGetRepository struct {
	domain.Repository
	target  string
	armed   atomic.Bool
	calls   atomic.Int32
	arrived chan struct{}
	release chan struct{}
}

func (r *synchronizedGetRepository) Get(ctx context.Context, id string) (*domain.ClearanceCase, error) {
	c, err := r.Repository.Get(ctx, id)
	if err != nil || id != r.target || !r.armed.Load() {
		return c, err
	}
	if r.calls.Add(1) <= 2 {
		r.arrived <- struct{}{}
		<-r.release
	}
	return c, nil
}

func TestConcurrentIdenticalChangeIsIdempotent(t *testing.T) {
	fileStore, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := &synchronizedGetRepository{
		Repository: fileStore,
		arrived:    make(chan struct{}, 2),
		release:    make(chan struct{}),
	}
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	service := application.NewService(repo, fileStore, rules.NewDefaultEngine(), application.FixedClock{Value: now}, nil)
	actor := application.Actor{Name: "技术总监", Role: domain.RoleTechnicalDirector}
	created, err := service.CreateCase(context.Background(), actor, application.CreateCaseCommand{
		RequestID:       "create",
		PerformanceName: "并发幂等测试",
		VenueZone:       "main",
		StartsAt:        now.Add(time.Hour),
		EndsAt:          now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	repo.target = created.ID
	repo.armed.Store(true)
	cmd := application.ReplacePlanCommand{
		RequestID:        "same-change",
		ExpectedRevision: created.Revision,
		Steps: []domain.MotionStep{{
			ID: "step-1", Sequence: 1, DeviceCode: "HOIST-A", Zone: "main", DurationMS: 1000,
		}},
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := service.ReplacePlan(context.Background(), created.ID, actor, cmd)
			results <- err
		}()
	}
	<-repo.arrived
	<-repo.arrived
	close(repo.release)

	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatalf("TestConcurrentIdenticalChangeIsIdempotent: 同一 request_id 的并发重放返回错误: %v", err)
		}
	}
}
