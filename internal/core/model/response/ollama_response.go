package response

// OllamaResponse represents the response format for /api/tags endpoint
type OllamaResponse struct {
	Models []OllamaModel `json:"models"`
}

// OllamaModel represents a model in the Ollama format
type OllamaModel struct {
	Name         string            `json:"name"`
	Model        string            `json:"model"`
	ModifiedAt   string            `json:"modified_at"`
	Size         int64             `json:"size"`
	Digest       string            `json:"digest"`
	Details      Details           `json:"details"`
	Capabilities []string          `json:"capabilities"`
	ModelInfo    map[string]string `json:"model_info"`
}

// Details represents the details of a model
type Details struct {
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// OllamaVersionResponse represents the response format for /api/version endpoint
type OllamaVersionResponse struct {
	Version string `json:"version"`
}
