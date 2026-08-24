package domain

import (
	"errors"
	"testing"
	"time"
)

func testCase(t *testing.T) *ClearanceCase {
	t.Helper()
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	c, err := NewCase(NewCaseInput{
		ID: "case-test", ClearanceNumber: "SC-TEST", PerformanceName: "测试演出",
		VenueZone: "main", StartsAt: now.Add(time.Hour), EndsAt: now.Add(3 * time.Hour),
		CreatedBy: "技术总监", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClearanceCaseWorkflow(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	c := testCase(t)
	steps := []MotionStep{{ID: "step-1", Sequence: 1, DeviceCode: "HOIST-A", Zone: "main", DurationMS: 5000, LoadKG: 600}}
	if err := c.ReplacePlan(steps, "技术总监", now); err != nil {
		t.Fatal(err)
	}
	if err := c.SubmitEvaluation("技术总监", now); err != nil {
		t.Fatal(err)
	}
	findings := []RiskFinding{{ID: "finding-1", MotionStepID: "step-1", RuleCode: "LOAD_LIMIT", Severity: SeverityCritical, Message: "超载"}}
	if err := c.ApplyEvaluation(findings, "rules-v1", "技术总监", now); err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusRemediation {
		t.Fatalf("状态=%s，期望=%s", c.Status, StatusRemediation)
	}
	if err := c.RequestReview("机械主管", now); !errors.Is(err, ErrEvidenceRequired) {
		t.Fatalf("缺少证据应被拒绝，得到 %v", err)
	}
	evidence := EvidenceRecord{ID: "evidence-1", OriginalName: "proof.txt", MediaType: "text/plain", SizeBytes: 5, SHA256: "abc", StorageKey: "ab/abc", Note: "整改完成"}
	if err := c.AddEvidence("finding-1", evidence, "机械主管", now); err != nil {
		t.Fatal(err)
	}
	if err := c.RequestReview("机械主管", now); err != nil {
		t.Fatal(err)
	}
	if err := c.ReviewFinding("finding-1", true, "证据有效", "复核员", now); err != nil {
		t.Fatal(err)
	}
	cert, err := NewCertificate("cert-1", c, "复核员", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.MarkReleased(cert, "复核员", now); err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusReleased || !VerifyCertificate(*c.Certificate) {
		t.Fatal("签署后状态或凭证无效")
	}
	if err := c.ReplacePlan(steps, "技术总监", now); !errors.Is(err, ErrAlreadyReleased) {
		t.Fatalf("已放行方案应锁定，得到 %v", err)
	}
}

func TestPlanDigestCanonicalizesInterlocks(t *testing.T) {
	c := testCase(t)
	c.RuleVersion = "v1"
	c.Steps = []MotionStep{{ID: "step-1", Sequence: 1, DeviceCode: "HOIST-A", Zone: "main", DurationMS: 5, InterlockCodes: []string{"UPPER-LIMIT", "E-STOP"}}}
	first, err := PlanDigest(c)
	if err != nil {
		t.Fatal(err)
	}
	c.Steps[0].InterlockCodes = []string{"E-STOP", "UPPER-LIMIT"}
	second, err := PlanDigest(c)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("规范化摘要不稳定: %s != %s", first, second)
	}
}

func TestReviewReturnMovesBackToRemediation(t *testing.T) {
	now := time.Now().UTC()
	c := testCase(t)
	c.Status = StatusPendingReview
	c.Findings = []RiskFinding{{ID: "finding-1", Status: FindingEvidence, Evidence: &EvidenceRecord{ID: "ev"}}}
	if err := c.ReviewFinding("finding-1", false, "现场记录不完整", "复核员", now); err != nil {
		t.Fatal(err)
	}
	if c.Status != StatusRemediation || c.Findings[0].Status != FindingReturned {
		t.Fatalf("退回状态不正确: case=%s finding=%s", c.Status, c.Findings[0].Status)
	}
}

func TestAcceptedFindingSurvivesAnotherFindingReturn(t *testing.T) {
	now := time.Now().UTC()
	c := testCase(t)
	c.Status = StatusPendingReview
	c.Findings = []RiskFinding{
		{ID: "accepted", Status: FindingAccepted, Evidence: &EvidenceRecord{ID: "ev-1"}},
		{ID: "returned", Status: FindingEvidence, Evidence: &EvidenceRecord{ID: "ev-2"}},
	}
	if err := c.ReviewFinding("returned", false, "补充现场照片", "复核员", now); err != nil {
		t.Fatal(err)
	}
	if err := c.AddEvidence("returned", EvidenceRecord{ID: "ev-3", SizeBytes: 3, SHA256: "abc", StorageKey: "ab/abc", Note: "已补充"}, "机械主管", now); err != nil {
		t.Fatal(err)
	}
	if err := c.RequestReview("机械主管", now); err != nil {
		t.Fatalf("已接受项不应阻止再次申请复核: %v", err)
	}
}
