package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"stage-clearance/internal/application"
	"stage-clearance/internal/domain"
	"stage-clearance/internal/rules"
	"stage-clearance/internal/store"
)

func runSelfTest(config Config, logger *slog.Logger) error {
	if config.DataDir == "data" {
		tempDir, err := os.MkdirTemp("", "stage-clearance-selftest-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tempDir)
		config.DataDir = tempDir
	}
	runtime, err := BuildRuntime(config, logger)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", config.Address)
	if err != nil {
		return fmt.Errorf("自检监听 %s: %w", config.Address, err)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- runtime.Server.Serve(listener) }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client := newSmokeClient(config.Address)
	flowErr := executeSmokeFlow(ctx, client, config.DataDir)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 4*time.Second)
	shutdownErr := runtime.Server.Shutdown(shutdownCtx)
	shutdownCancel()
	serverErr := <-serveErr
	if flowErr != nil {
		return flowErr
	}
	if shutdownErr != nil {
		return shutdownErr
	}
	if serverErr != nil && serverErr != http.ErrServerClosed {
		return serverErr
	}
	logger.Info("自检完成", "address", config.Address, "result", "完整放行闭环及重启恢复验证通过")
	return nil
}

func executeSmokeFlow(ctx context.Context, client *smokeClient, dataDir string) error {
	if err := client.waitReady(ctx); err != nil {
		return err
	}
	technical := application.Actor{Name: "自检技术总监", Role: domain.RoleTechnicalDirector}
	mechanical := application.Actor{Name: "自检机械主管", Role: domain.RoleMechanicalLead}
	reviewer := application.Actor{Name: "自检安全复核员", Role: domain.RoleSafetyReviewer}
	start := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	var caseView application.CaseView
	err := client.json(ctx, http.MethodPost, "/api/cases", technical, application.CreateCaseCommand{
		RequestID: client.requestID("create"), PerformanceName: "自检演出：机械联排",
		VenueZone: "main", StartsAt: start, EndsAt: start.Add(2 * time.Hour),
	}, &caseView)
	if err != nil {
		return fmt.Errorf("自检建单: %w", err)
	}
	path := "/api/cases/" + url.PathEscape(caseView.ID) + "/plan"
	err = client.json(ctx, http.MethodPut, path, technical, application.ReplacePlanCommand{
		RequestID: client.requestID("plan"), ExpectedRevision: caseView.Revision, Steps: smokeSteps(),
	}, &caseView)
	if err != nil {
		return fmt.Errorf("自检保存方案: %w", err)
	}
	path = "/api/cases/" + url.PathEscape(caseView.ID) + "/evaluate"
	err = client.json(ctx, http.MethodPost, path, technical, application.EvaluateCommand{
		RequestID: client.requestID("evaluate"), ExpectedRevision: caseView.Revision,
	}, &caseView)
	if err != nil {
		return fmt.Errorf("自检规则评估: %w", err)
	}
	if caseView.Status != domain.StatusRemediation || len(caseView.Findings) < 4 {
		return fmt.Errorf("自检规则评估未生成预期风险: status=%s findings=%d", caseView.Status, len(caseView.Findings))
	}
	for _, finding := range caseView.Findings {
		caseView, err = client.evidence(ctx, caseView.ID, finding.ID, caseView.Revision, mechanical)
		if err != nil {
			return fmt.Errorf("自检提交证据 %s: %w", finding.ID, err)
		}
	}
	path = "/api/cases/" + url.PathEscape(caseView.ID) + "/review-request"
	err = client.json(ctx, http.MethodPost, path, mechanical, application.RequestReviewCommand{
		RequestID: client.requestID("review-request"), ExpectedRevision: caseView.Revision,
	}, &caseView)
	if err != nil {
		return fmt.Errorf("自检申请复核: %w", err)
	}
	for _, finding := range caseView.Findings {
		path = fmt.Sprintf("/api/cases/%s/findings/%s/review", url.PathEscape(caseView.ID), url.PathEscape(finding.ID))
		err = client.json(ctx, http.MethodPost, path, reviewer, application.ReviewFindingCommand{
			RequestID: client.requestID("review"), ExpectedRevision: caseView.Revision,
			Accepted: true, Note: "证据摘要、现场复测记录与规则定位一致，同意接受。",
		}, &caseView)
		if err != nil {
			return fmt.Errorf("自检复核 %s: %w", finding.ID, err)
		}
	}
	path = "/api/cases/" + url.PathEscape(caseView.ID) + "/sign"
	var certificate application.CertificateView
	err = client.json(ctx, http.MethodPost, path, reviewer, application.SignCommand{
		RequestID: client.requestID("sign"), ExpectedRevision: caseView.Revision,
	}, &certificate)
	if err != nil {
		return fmt.Errorf("自检签署: %w", err)
	}
	if !certificate.Valid || !domain.VerifyCertificate(certificate.Certificate) {
		return fmt.Errorf("自检签署生成了无效凭证")
	}
	query := url.Values{
		"clearance_number":  []string{certificate.Certificate.ClearanceNumber},
		"verification_code": []string{certificate.Certificate.VerificationCode},
	}
	var verified application.CertificateView
	if err := client.json(ctx, http.MethodGet, "/api/certificates/verify?"+query.Encode(), application.Actor{}, nil, &verified); err != nil {
		return fmt.Errorf("自检公开凭证核验: %w", err)
	}
	var timeline application.TimelineView
	if err := client.json(ctx, http.MethodGet, "/api/cases/"+url.PathEscape(caseView.ID)+"/timeline", application.Actor{}, nil, &timeline); err != nil {
		return fmt.Errorf("自检读取时间线: %w", err)
	}
	if len(timeline.Events) < 10 {
		return fmt.Errorf("自检审计事件不完整: %d", len(timeline.Events))
	}
	reloadedStore, err := store.New(dataDir)
	if err != nil {
		return fmt.Errorf("自检重新加载存储: %w", err)
	}
	reloadedService := application.NewService(reloadedStore, reloadedStore, rules.NewDefaultEngine(), application.SystemClock{}, application.RandomIDGenerator{})
	reloaded, err := reloadedService.VerifyCertificate(ctx, application.CertificateLookup{
		ClearanceNumber:  certificate.Certificate.ClearanceNumber,
		VerificationCode: certificate.Certificate.VerificationCode,
	})
	if err != nil || !reloaded.Valid {
		return fmt.Errorf("自检重启恢复凭证: %w", err)
	}
	return nil
}
