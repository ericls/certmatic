package domain

type DNSRecord struct {
	Type  string `json:"type" yaml:"type"`
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}
