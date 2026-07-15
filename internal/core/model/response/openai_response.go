package response

// OpenAIErrorResponse represents the error envelope returned by the OpenAI API
type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

// OpenAIError represents the error detail in the OpenAI format
type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// OpenAIListResponse represents the response format for GET /v1/models endpoint
type OpenAIListResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// OpenAIModel represents a model in the OpenAI format
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}
