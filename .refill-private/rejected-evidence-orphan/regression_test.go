package regression

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"stage-clearance/internal/application"
	"stage-clearance/internal/domain"
	"stage-clearance/internal/store"
)

type oneFindingEvaluator struct{}

func (oneFindingEvaluator) Version() string { return "private-test-v1" }

func (oneFindingEvaluator) Evaluate(context.Context, *domain.ClearanceCase) ([]domain.FindingSpec, error) {
	return []domain.FindingSpec{{
		Key: "finding-key", MotionStepID: "step-1", RuleCode: "PRIVATE_RULE",
		Severity: domain.SeverityHigh, Message: "测试风险", Location: "step-1",
	}}, nil
}

func TestRejectedEvidenceMustNotLeaveBlob(t *testing.T) {
	root := t.TempDir()
	repo, err := store.New(root)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	service := application.NewService(repo, repo, oneFindingEvaluator{}, application.FixedClock{Value: now}, nil)
	technical := application.Actor{Name: "技术总监", Role: domain.RoleTechnicalDirector}
	mechanical := application.Actor{Name: "机械主管", Role: domain.RoleMechanicalLead}
	ctx := context.Background()
	view, err := service.CreateCase(ctx, technical, application.CreateCaseCommand{
		RequestID: "create", PerformanceName: "证据事务测试", VenueZone: "main",
		StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.ReplacePlan(ctx, view.ID, technical, application.ReplacePlanCommand{
		RequestID: "plan", ExpectedRevision: view.Revision,
		Steps: []domain.MotionStep{{ID: "step-1", Sequence: 1, DeviceCode: "HOIST-A", Zone: "main", DurationMS: 1000}},
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Evaluate(ctx, view.ID, technical, application.EvaluateCommand{
		RequestID: "evaluate", ExpectedRevision: view.Revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.SubmitEvidence(ctx, view.ID, mechanical, application.SubmitEvidenceCommand{
		RequestID: "invalid-evidence", ExpectedRevision: view.Revision,
		FindingID: "missing-finding", OriginalName: "proof.txt", MediaType: "text/plain",
		Note: "整改完成", Content: []byte("unreferenced evidence"),
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("未进入预期的 finding 拒绝路径: %v", err)
	}

	files := 0
	err = filepath.WalkDir(filepath.Join(root, "evidence"), func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type().IsRegular() {
			files++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files != 0 {
		t.Fatalf("TestRejectedEvidenceMustNotLeaveBlob: 业务拒绝后遗留了 %d 个无案件引用的附件", files)
	}
}
