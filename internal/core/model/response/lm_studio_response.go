package response

// LMStudioListResponse represents the response format for GET /api/v0/models endpoint
type LMStudioListResponse struct {
	Object string          `json:"object"`
	Data   []LMStudioModel `json:"data"`
}

// LMStudioModel represents a model in the LM Studio format
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
