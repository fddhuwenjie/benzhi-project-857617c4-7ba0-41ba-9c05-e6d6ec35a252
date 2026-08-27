package evaluationflightcontext_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"stage-clearance/internal/domain"
	"stage-clearance/internal/rules"
)

type blockingErrContext struct {
	context.Context
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (c *blockingErrContext) Err() error {
	if c.calls.Add(1) == 2 {
		close(c.entered)
		<-c.release
	}
	return c.Context.Err()
}

type observedDoneContext struct {
	context.Context
	once     sync.Once
	observed chan struct{}
}

func (c *observedDoneContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

type evaluationResult struct {
	findings []domain.FindingSpec
	err      error
}

func TestConcurrentEvaluationMustNotInheritLeaderCancellation(t *testing.T) {
	engine := rules.NewDefaultEngine()
	clearanceCase := evaluationCase(t)

	leaderBase, cancelLeader := context.WithCancel(context.Background())
	leaderCtx := &blockingErrContext{
		Context: leaderBase,
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	leaderResult := make(chan evaluationResult, 1)
	go func() {
		findings, err := engine.Evaluate(leaderCtx, clearanceCase)
		leaderResult <- evaluationResult{findings: findings, err: err}
	}()
	<-leaderCtx.entered

	followerCtx := &observedDoneContext{Context: context.Background(), observed: make(chan struct{})}
	followerResult := make(chan evaluationResult, 1)
	go func() {
		findings, err := engine.Evaluate(followerCtx, clearanceCase)
		followerResult <- evaluationResult{findings: findings, err: err}
	}()

	var completedFollower *evaluationResult
	select {
	case <-followerCtx.observed:
	case result := <-followerResult:
		completedFollower = &result
	}
	cancelLeader()
	close(leaderCtx.release)

	if result := <-leaderResult; !errors.Is(result.err, context.Canceled) {
		t.Fatalf("首个评估应响应自身取消，得到 findings=%v err=%v", result.findings, result.err)
	}
	var follower evaluationResult
	if completedFollower != nil {
		follower = *completedFollower
	} else {
		follower = <-followerResult
	}
	if follower.err != nil {
		t.Fatalf("仍存活的并发评估继承了首请求取消: %v", follower.err)
	}
	if len(follower.findings) == 0 {
		t.Fatal("仍存活的并发评估应返回确定性风险项")
	}
}

func evaluationCase(t *testing.T) *domain.ClearanceCase {
	t.Helper()
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	c, err := domain.NewCase(domain.NewCaseInput{
		ID: "case-flight-context", ClearanceNumber: "SC-FLIGHT-CONTEXT",
		PerformanceName: "并发评估", VenueZone: "main",
		StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour),
		CreatedBy: "技术总监", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = c.ReplacePlan([]domain.MotionStep{{
		ID: "step-flight-context", Sequence: 1, DeviceCode: "HOIST-A", Zone: "main",
		DurationMS: 5000, LoadKG: 900, RequiresClearance: true,
	}}, "技术总监", now)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
