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

// LMStudioV1ListResponse represents the /api/v1/models response
type LMStudioV1ListResponse struct {
	Models []LMStudioV1Model `json:"models"`
}

// LMStudioV1Model represents a model in the LM Studio v1 format.
// Reconstructed from the local store, so it is best-effort: capabilities.reasoning is
// omitted because entity.Model stores capabilities in Ollama's vocabulary, which is not
// reversible into the full v1 object.
type LMStudioV1Model struct {
	Key              string                     `json:"key"`
	DisplayName      string                     `json:"display_name"`
	Type             string                     `json:"type"`
	Publisher        string                     `json:"publisher"`
	Architecture     string                     `json:"architecture,omitempty"`
	Quantization     *LMStudioV1Quantization    `json:"quantization"`
	Format           string                     `json:"format,omitempty"`
	SizeBytes        int64                      `json:"size_bytes"`
	ParamsString     string                     `json:"params_string,omitempty"`
	MaxContextLength int                        `json:"max_context_length"`
	LoadedInstances  []LMStudioV1LoadedInstance `json:"loaded_instances"`
	Capabilities     *LMStudioV1Capabilities    `json:"capabilities,omitempty"`
}

// LMStudioV1Quantization represents the v1 quantization object
type LMStudioV1Quantization struct {
	Name string `json:"name"`
}

// LMStudioV1Capabilities represents the v1 capabilities object
type LMStudioV1Capabilities struct {
	Vision            bool `json:"vision"`
	TrainedForToolUse bool `json:"trained_for_tool_use"`
}

// LMStudioV1LoadedInstance represents a loaded instance of a model
type LMStudioV1LoadedInstance struct {
	ID string `json:"id"`
}
