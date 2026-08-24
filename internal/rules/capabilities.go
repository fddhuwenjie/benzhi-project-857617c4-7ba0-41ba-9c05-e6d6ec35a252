package rules

import "stage-clearance/internal/domain"

type DeviceCapability struct {
	Code               string
	MaxLoadKG          float64
	RequiredInterlocks []string
	AllowedZones       []string
}

func DefaultCapabilities() map[string]DeviceCapability {
	values := []DeviceCapability{
		{Code: "HOIST-A", MaxLoadKG: 500, RequiredInterlocks: []string{"E-STOP", "UPPER-LIMIT"}, AllowedZones: []string{"main", "fly"}},
		{Code: "HOIST-B", MaxLoadKG: 800, RequiredInterlocks: []string{"E-STOP", "UPPER-LIMIT"}, AllowedZones: []string{"main", "fly"}},
		{Code: "LIFT-1", MaxLoadKG: 1000, RequiredInterlocks: []string{"E-STOP", "PIT-GATE"}, AllowedZones: []string{"main", "pit"}},
		{Code: "REVOLVE-1", MaxLoadKG: 1200, RequiredInterlocks: []string{"E-STOP", "EDGE-SENSOR"}, AllowedZones: []string{"main"}},
		{Code: "TRACK-1", MaxLoadKG: 300, RequiredInterlocks: []string{"E-STOP", "TRACK-LIMIT"}, AllowedZones: []string{"main", "wing-left", "wing-right"}},
	}
	out := make(map[string]DeviceCapability, len(values))
	for _, value := range values {
		out[value.Code] = value
	}
	return out
}

func SupportsZone(capability DeviceCapability, zone string) bool {
	for _, allowed := range capability.AllowedZones {
		if allowed == zone {
			return true
		}
	}
	return false
}

func severityRank(value domain.Severity) int {
	switch value {
	case domain.SeverityCritical:
		return 0
	case domain.SeverityHigh:
		return 1
	case domain.SeverityMedium:
		return 2
	default:
		return 3
	}
}
