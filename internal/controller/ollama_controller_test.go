package controller

import (
	"testing"

	"fernandoglatz/openai-compatible-proxy/internal/core/entity"
)

// When the v1 sync populated the store, the Ollama view must report what LM Studio
// actually said rather than guessing from the model name.
func TestOllamaUsesStoredV1Metadata(t *testing.T) {
	controller := NewOllamaController(nil, nil)

	result := controller.convertToOllamaResponse([]entity.Model{{
		Name:         "google/gemma-4-26b-a4b",
		Type:         "llm",
		SizeBytes:    16000000000,
		ParamsString: "26B-A4B",
		Capabilities: []string{"completion", "chat", "tools", "vision", "thinking"},
	}})

	model := result.Models[0]

	if model.Size != 16000000000 {
		t.Errorf("Size = %d, want the stored SizeBytes (was hardcoded 0)", model.Size)
	}
	if model.Details.ParameterSize != "26B-A4B" {
		t.Errorf("ParameterSize = %q, want the stored ParamsString; the regex would have said \"26B\"", model.Details.ParameterSize)
	}
	if len(model.Capabilities) != 5 {
		t.Errorf("Capabilities = %v, want the stored five", model.Capabilities)
	}
}

// The v0 sync path stores none of that metadata, so the old guessing must still apply.
func TestOllamaFallsBackWhenV1MetadataAbsent(t *testing.T) {
	controller := NewOllamaController(nil, nil)

	result := controller.convertToOllamaResponse([]entity.Model{{
		Name: "meta/llama-3-8b",
		Type: "llm",
	}})

	model := result.Models[0]

	if model.Details.ParameterSize != "8B" {
		t.Errorf("ParameterSize = %q, want \"8B\" from the name-regex fallback", model.Details.ParameterSize)
	}
	if len(model.Capabilities) == 0 {
		t.Error("Capabilities is empty; the type-based fallback must still populate it")
	}
	if model.Size != 0 {
		t.Errorf("Size = %d, want 0 when SizeBytes is unknown", model.Size)
	}
}
