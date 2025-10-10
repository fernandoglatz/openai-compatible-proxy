package request

type ModelRequest struct {
	Name              string `json:"name"`
	Object            string `json:"object"`
	Type              string `json:"type"`
	Publisher         string `json:"publisher"`
	Arch              string `json:"arch"`
	CompatibilityType string `json:"compatibilityType"`
	Quantization      string `json:"quantization"`
	State             string `json:"state"`
	MaxContextLength  int    `json:"maxContextLength"`
}

// OllamaShowRequest represents the request format for /api/show endpoint
type OllamaShowRequest struct {
	Model string `json:"model" validate:"required"`
}
