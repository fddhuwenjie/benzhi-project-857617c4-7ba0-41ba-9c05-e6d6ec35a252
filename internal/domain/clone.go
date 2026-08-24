package domain

import "encoding/json"

func CloneCase(c *ClearanceCase) (*ClearanceCase, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var clone ClearanceCase
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}
	if clone.ProcessedRequests == nil {
		clone.ProcessedRequests = map[string]IdempotencyRecord{}
	}
	return &clone, nil
}

func CloneCertificate(cert *ReleaseCertificate) (*ReleaseCertificate, error) {
	if cert == nil {
		return nil, nil
	}
	data, err := json.Marshal(cert)
	if err != nil {
		return nil, err
	}
	var clone ReleaseCertificate
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}
