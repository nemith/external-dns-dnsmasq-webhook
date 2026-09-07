package main

type endpoint struct {
	DNSName          string                     `json:"dnsName,omitempty"`
	Targets          []string                   `json:"targets,omitempty"`
	RecordType       string                     `json:"recordType,omitempty"`
	SetIdentifier    string                     `json:"setIdentifier,omitempty"`
	RecordTTL        int64                      `json:"recordTTL,omitempty"`
	Labels           map[string]string          `json:"labels,omitempty"`
	ProviderSpecific []providerSpecificProperty `json:"providerSpecific,omitempty"`
}

type providerSpecificProperty struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type changes struct {
	Create    []*endpoint `json:"create,omitempty"`
	UpdateOld []*endpoint `json:"updateOld,omitempty"`
	UpdateNew []*endpoint `json:"updateNew,omitempty"`
	Delete    []*endpoint `json:"delete,omitempty"`
}

type domainFilter struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}
