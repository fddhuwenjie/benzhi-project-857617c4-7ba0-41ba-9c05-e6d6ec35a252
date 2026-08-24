package application

import (
	"context"
	"testing"
	"time"

	"stage-clearance/internal/domain"
	"stage-clearance/internal/rules"
	"stage-clearance/internal/store"
)

func TestServiceWorkflowAndIdempotency(t *testing.T) {
	repo, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	ids := &SequenceIDGenerator{Values: []string{"evidence-1", "evidence-2", "evidence-3", "certificate-1"}}
	service := NewService(repo, repo, rules.NewDefaultEngine(), FixedClock{Value: now}, ids)
	ctx := context.Background()
	technical := Actor{Name: "总监", Role: domain.RoleTechnicalDirector}
	mechanical := Actor{Name: "主管", Role: domain.RoleMechanicalLead}
	reviewer := Actor{Name: "复核员", Role: domain.RoleSafetyReviewer}
	create := CreateCaseCommand{RequestID: "request-create", PerformanceName: "应用测试", VenueZone: "main", StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour)}
	view, err := service.CreateCase(ctx, technical, create)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := service.CreateCase(ctx, technical, create)
	if err != nil || repeated.ID != view.ID {
		t.Fatalf("建单幂等失败: %#v %v", repeated, err)
	}
	view, err = service.ReplacePlan(ctx, view.ID, technical, ReplacePlanCommand{RequestID: "request-plan", ExpectedRevision: view.Revision, Steps: []domain.MotionStep{{ID: "step-1", Sequence: 1, DeviceCode: "HOIST-A", Zone: "main", DurationMS: 5000, LoadKG: 700, RequiresClearance: true, InterlockCodes: []string{"E-STOP"}}}})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Evaluate(ctx, view.ID, technical, EvaluateCommand{RequestID: "request-eval", ExpectedRevision: view.Revision})
	if err != nil {
		t.Fatal(err)
	}
	for index, finding := range view.Findings {
		view, err = service.SubmitEvidence(ctx, view.ID, mechanical, SubmitEvidenceCommand{RequestID: "request-evidence-" + finding.ID, ExpectedRevision: view.Revision, FindingID: finding.ID, OriginalName: "proof.txt", MediaType: "text/plain", Note: "整改完成", Content: []byte("proof")})
		if err != nil {
			t.Fatalf("证据 %d: %v", index, err)
		}
	}
	view, err = service.RequestReview(ctx, view.ID, mechanical, RequestReviewCommand{RequestID: "request-review", ExpectedRevision: view.Revision})
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range view.Findings {
		view, err = service.ReviewFinding(ctx, view.ID, reviewer, ReviewFindingCommand{RequestID: "request-accept-" + finding.ID, ExpectedRevision: view.Revision, FindingID: finding.ID, Accepted: true, Note: "接受"})
		if err != nil {
			t.Fatal(err)
		}
	}
	cert, err := service.Sign(ctx, view.ID, reviewer, SignCommand{RequestID: "request-sign", ExpectedRevision: view.Revision})
	if err != nil || !cert.Valid {
		t.Fatalf("签署失败: %#v %v", cert, err)
	}
	verified, err := service.VerifyCertificate(ctx, CertificateLookup{ClearanceNumber: cert.Certificate.ClearanceNumber, VerificationCode: cert.Certificate.VerificationCode})
	if err != nil || !verified.Valid {
		t.Fatalf("凭证核验失败: %#v %v", verified, err)
	}
}
