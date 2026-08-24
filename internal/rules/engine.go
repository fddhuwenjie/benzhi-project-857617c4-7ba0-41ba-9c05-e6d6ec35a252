package rules

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"stage-clearance/internal/domain"
)

const RuleVersion = "SC-RULES-2026.1"

type Engine struct {
	capabilities map[string]DeviceCapability
	cacheMu      sync.RWMutex
	cache        map[string][]domain.FindingSpec
}

func NewEngine(capabilities map[string]DeviceCapability) *Engine {
	copyCapabilities := make(map[string]DeviceCapability, len(capabilities))
	for code, capability := range capabilities {
		copyCapabilities[code] = capability
	}
	return &Engine{
		capabilities: copyCapabilities,
		cache:        make(map[string][]domain.FindingSpec),
	}
}

func NewDefaultEngine() *Engine {
	return NewEngine(DefaultCapabilities())
}

func (e *Engine) Version() string { return RuleVersion }

func (e *Engine) Evaluate(ctx context.Context, c *domain.ClearanceCase) ([]domain.FindingSpec, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c == nil {
		return nil, domain.NewValidation("case", "不能为空")
	}
	if err := domain.ValidateSteps(c.ID, c.StartsAt, c.EndsAt, c.Steps); err != nil {
		return nil, err
	}
	cacheKey, err := domain.PlanDigest(c)
	if err != nil {
		return nil, err
	}
	e.cacheMu.RLock()
	cached, ok := e.cache[cacheKey]
	e.cacheMu.RUnlock()
	if ok {
		return cached, nil
	}
	findings := make([]domain.FindingSpec, 0)
	for _, step := range c.Steps {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		capability, ok := e.capabilities[step.DeviceCode]
		if !ok {
			return nil, fmt.Errorf("%w: 未知设备能力 %s", domain.ErrValidation, step.DeviceCode)
		}
		if !SupportsZone(capability, step.Zone) {
			return nil, fmt.Errorf("%w: 设备 %s 缺少区域 %s 的规则输入", domain.ErrValidation, step.DeviceCode, step.Zone)
		}
		if step.LoadKG > capability.MaxLoadKG {
			findings = append(findings, domain.FindingSpec{
				Key: fmt.Sprintf("load:%s", step.ID), MotionStepID: step.ID,
				RuleCode: "LOAD_LIMIT", Severity: domain.SeverityCritical,
				Message:  fmt.Sprintf("计划载荷 %.1fkg 超过设备 %s 上限 %.1fkg", step.LoadKG, step.DeviceCode, capability.MaxLoadKG),
				Location: location(step),
			})
		}
		if step.RequiresClearance && !step.ClearanceConfirmed {
			findings = append(findings, domain.FindingSpec{
				Key: fmt.Sprintf("clearance:%s", step.ID), MotionStepID: step.ID,
				RuleCode: "PERSONNEL_CLEARANCE", Severity: domain.SeverityHigh,
				Message:  "动作要求人员净空，但计划未确认净空窗口",
				Location: location(step),
			})
		}
		provided := stringSet(step.InterlockCodes)
		for _, required := range capability.RequiredInterlocks {
			if !provided[required] {
				findings = append(findings, domain.FindingSpec{
					Key: fmt.Sprintf("interlock:%s:%s", step.ID, required), MotionStepID: step.ID,
					RuleCode: "INTERLOCK_REQUIRED", Severity: domain.SeverityHigh,
					Message:  fmt.Sprintf("设备 %s 动作缺少互锁前置条件 %s", step.DeviceCode, required),
					Location: location(step),
				})
			}
		}
	}
	findings = append(findings, overlappingZoneFindings(c.Steps)...)
	sortFindings(findings, c.Steps)
	e.cacheMu.Lock()
	e.cache[cacheKey] = findings
	e.cacheMu.Unlock()
	return findings, nil
}

func overlappingZoneFindings(steps []domain.MotionStep) []domain.FindingSpec {
	out := make([]domain.FindingSpec, 0)
	ordered := append([]domain.MotionStep(nil), steps...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].StartsAtOffsetMS == ordered[j].StartsAtOffsetMS {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].StartsAtOffsetMS < ordered[j].StartsAtOffsetMS
	})
	for i := 0; i < len(ordered); i++ {
		for j := i + 1; j < len(ordered); j++ {
			left, right := ordered[i], ordered[j]
			if right.StartsAtOffsetMS >= left.EndsAtOffsetMS() {
				break
			}
			if left.Zone != right.Zone || left.DeviceCode == right.DeviceCode {
				continue
			}
			ids := []string{left.ID, right.ID}
			sort.Strings(ids)
			out = append(out, domain.FindingSpec{
				Key: "zone:" + strings.Join(ids, ":"), MotionStepID: right.ID,
				RuleCode: "ZONE_CONFLICT", Severity: domain.SeverityCritical,
				Message:  fmt.Sprintf("区域 %s 内设备 %s 与 %s 的运动时间重叠", right.Zone, left.DeviceCode, right.DeviceCode),
				Location: fmt.Sprintf("动作 %s/%s，%d-%dms", left.ID, right.ID, right.StartsAtOffsetMS, min(left.EndsAtOffsetMS(), right.EndsAtOffsetMS())),
			})
		}
	}
	return out
}

func sortFindings(findings []domain.FindingSpec, steps []domain.MotionStep) {
	sequence := make(map[string]int, len(steps))
	for _, step := range steps {
		sequence[step.ID] = step.Sequence
	}
	sort.Slice(findings, func(i, j int) bool {
		if severityRank(findings[i].Severity) != severityRank(findings[j].Severity) {
			return severityRank(findings[i].Severity) < severityRank(findings[j].Severity)
		}
		if sequence[findings[i].MotionStepID] != sequence[findings[j].MotionStepID] {
			return sequence[findings[i].MotionStepID] < sequence[findings[j].MotionStepID]
		}
		if findings[i].RuleCode != findings[j].RuleCode {
			return findings[i].RuleCode < findings[j].RuleCode
		}
		return findings[i].Key < findings[j].Key
	})
}

func location(step domain.MotionStep) string {
	return fmt.Sprintf("动作 %s（序号 %d，设备 %s，区域 %s，%d-%dms）", step.ID, step.Sequence, step.DeviceCode, step.Zone, step.StartsAtOffsetMS, step.EndsAtOffsetMS())
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[strings.TrimSpace(value)] = true
	}
	return out
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
