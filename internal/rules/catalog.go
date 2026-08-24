package rules

type RuleDescription struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func Catalog() []RuleDescription {
	return []RuleDescription{
		{Code: "LOAD_LIMIT", Name: "设备载荷上限", Description: "动作载荷不得超过登记设备的额定安全载荷。"},
		{Code: "ZONE_CONFLICT", Name: "运动区域冲突", Description: "不同机械设备不得在同一区域同时运动。"},
		{Code: "PERSONNEL_CLEARANCE", Name: "人员净空窗口", Description: "标记为需要净空的动作必须在方案中确认净空。"},
		{Code: "INTERLOCK_REQUIRED", Name: "互锁前置条件", Description: "动作必须声明设备能力表要求的全部安全互锁。"},
	}
}
