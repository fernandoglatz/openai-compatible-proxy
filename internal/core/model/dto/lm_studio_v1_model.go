package dto

// LMStudioV1Response represents the /api/v1/models response from LM Studio 0.4.0+
type LMStudioV1Response struct {
	Models []LMStudioV1Model `json:"models"`
}

// LMStudioV1Model represents a model as returned by /api/v1/models.
// Nullable fields are pointers so an explicit null is distinguishable from a zero value.
type LMStudioV1Model struct {
	Key              string               `json:"key"`
	DisplayName      string               `json:"display_name"`
	Type             string               `json:"type"`
	Publisher        string               `json:"publisher"`
	Architecture     *string              `json:"architecture"`
	Quantization     *LMStudioV1Quant     `json:"quantization"`
	Format           *string              `json:"format"`
	SizeBytes        int64                `json:"size_bytes"`
	ParamsString     *string              `json:"params_string"`
	MaxContextLength int                  `json:"max_context_length"`
	LoadedInstances  []LMStudioV1Instance `json:"loaded_instances"`
	Capabilities     *LMStudioV1Caps      `json:"capabilities"`
}

// LMStudioV1Quant represents the v1 quantization object, which replaced v0's plain string
type LMStudioV1Quant struct {
	Name          *string  `json:"name"`
	BitsPerWeight *float64 `json:"bits_per_weight"`
}

// LMStudioV1Caps represents the v1 capabilities object. Absent for embedding models.
type LMStudioV1Caps struct {
	Vision            bool                 `json:"vision"`
	TrainedForToolUse bool                 `json:"trained_for_tool_use"`
	Reasoning         *LMStudioV1Reasoning `json:"reasoning"`
}

// LMStudioV1Reasoning describes a model's reasoning options
type LMStudioV1Reasoning struct {
	AllowedOptions []string `json:"allowed_options"`
	Default        string   `json:"default"`
}

// LMStudioV1Instance represents a loaded instance of a model
type LMStudioV1Instance struct {
	ID string `json:"id"`
}
