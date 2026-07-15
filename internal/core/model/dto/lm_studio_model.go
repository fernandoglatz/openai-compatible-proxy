package dto

// LMStudioModel represents a model structure returned by LM Studio API.
// The json tags map the v0 wire format. The trailing fields are v1-only: they are
// populated by the v1 normalizer and stay zero when v0 answered, so they are tagged
// json:"-" to keep them out of the v0 contract.
type LMStudioModel struct {
	ID                string `json:"id"`
	Object            string `json:"object"`
	Type              string `json:"type"`
	Publisher         string `json:"publisher"`
	Arch              string `json:"arch"`
	CompatibilityType string `json:"compatibility_type"`
	Quantization      string `json:"quantization"`
	State             string `json:"state"`
	MaxContextLength  int    `json:"max_context_length"`

	DisplayName       string   `json:"-"`
	SizeBytes         int64    `json:"-"`
	ParamsString      string   `json:"-"`
	Capabilities      []string `json:"-"`
	LoadedInstanceIDs []string `json:"-"`
}

// LMStudioResponse represents the structure of LM Studio API response
type LMStudioResponse struct {
	Data   []LMStudioModel `json:"data"`
	Object string          `json:"object"`
}
