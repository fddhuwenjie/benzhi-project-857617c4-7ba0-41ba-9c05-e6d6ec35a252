package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	MaxMotionSteps = 200
	MaxDurationMS  = int64(24 * time.Hour / time.Millisecond)
)

func ValidateSteps(caseID string, startsAt, endsAt time.Time, steps []MotionStep) error {
	fields := make([]FieldError, 0)
	if len(steps) == 0 {
		fields = append(fields, FieldError{Field: "steps", Message: "至少需要一个动作步骤"})
	}
	if len(steps) > MaxMotionSteps {
		fields = append(fields, FieldError{Field: "steps", Message: fmt.Sprintf("最多允许 %d 个动作步骤", MaxMotionSteps)})
	}
	windowMS := endsAt.Sub(startsAt).Milliseconds()
	ids := map[string]bool{}
	sequences := map[int]bool{}
	for i, step := range steps {
		prefix := fmt.Sprintf("steps[%d]", i)
		if strings.TrimSpace(step.ID) == "" {
			fields = append(fields, FieldError{Field: prefix + ".id", Message: "不能为空"})
		} else if ids[step.ID] {
			fields = append(fields, FieldError{Field: prefix + ".id", Message: "不能重复"})
		}
		ids[step.ID] = true
		if step.CaseID != "" && step.CaseID != caseID {
			fields = append(fields, FieldError{Field: prefix + ".case_id", Message: "引用了其他放行单"})
		}
		if step.Sequence <= 0 || sequences[step.Sequence] {
			fields = append(fields, FieldError{Field: prefix + ".sequence", Message: "必须为唯一正整数"})
		}
		sequences[step.Sequence] = true
		if strings.TrimSpace(step.DeviceCode) == "" {
			fields = append(fields, FieldError{Field: prefix + ".device_code", Message: "不能为空"})
		}
		if strings.TrimSpace(step.Zone) == "" {
			fields = append(fields, FieldError{Field: prefix + ".zone", Message: "不能为空"})
		}
		if step.StartsAtOffsetMS < 0 {
			fields = append(fields, FieldError{Field: prefix + ".starts_at_offset_ms", Message: "不能为负数"})
		}
		if step.DurationMS <= 0 || step.DurationMS > MaxDurationMS {
			fields = append(fields, FieldError{Field: prefix + ".duration_ms", Message: "必须处于 1 毫秒到 24 小时之间"})
		}
		if step.EndsAtOffsetMS() > windowMS {
			fields = append(fields, FieldError{Field: prefix + ".duration_ms", Message: "动作不得超出演出时段"})
		}
		if step.LoadKG < 0 {
			fields = append(fields, FieldError{Field: prefix + ".load_kg", Message: "不能为负数"})
		}
		seenInterlock := map[string]bool{}
		for _, code := range step.InterlockCodes {
			code = strings.TrimSpace(code)
			if code == "" || seenInterlock[code] {
				fields = append(fields, FieldError{Field: prefix + ".interlock_codes", Message: "互锁代码不能为空或重复"})
			}
			seenInterlock[code] = true
		}
	}
	return CombineValidation(fields...)
}

func ValidateRole(actual Role, allowed ...Role) error {
	for _, role := range allowed {
		if actual == role {
			return nil
		}
	}
	return ErrForbidden
}

func ValidateEvidenceMetadata(findingID, originalName, note string) error {
	fields := make([]FieldError, 0)
	if strings.TrimSpace(findingID) == "" {
		fields = append(fields, FieldError{Field: "finding_id", Message: "不能为空"})
	}
	if strings.TrimSpace(originalName) == "" {
		fields = append(fields, FieldError{Field: "original_name", Message: "不能为空"})
	}
	if strings.TrimSpace(note) == "" {
		fields = append(fields, FieldError{Field: "note", Message: "整改说明不能为空"})
	}
	return CombineValidation(fields...)
}

func ValidStatus(status CaseStatus) bool {
	switch status {
	case StatusDraft, StatusPendingEval, StatusRemediation, StatusPendingReview, StatusReleased:
		return true
	default:
		return false
	}
}
