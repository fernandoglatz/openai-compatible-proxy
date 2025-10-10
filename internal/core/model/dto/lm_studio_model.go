package dto

// LMStudioModel represents a model structure returned by LM Studio API
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
}

// LMStudioResponse represents the structure of LM Studio API response
type LMStudioResponse struct {
	Data   []LMStudioModel `json:"data"`
	Object string          `json:"object"`
}
