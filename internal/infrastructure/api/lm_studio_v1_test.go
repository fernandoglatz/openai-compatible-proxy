package api

import (
	"encoding/json"
	"reflect"
	"testing"

	"fernandoglatz/openai-compatible-proxy/internal/core/model/dto"
)

// fullV1Payload is a representative /api/v1/models body.
const fullV1Payload = `{
  "models": [
    {
      "key": "google/gemma-4-26b-a4b",
      "display_name": "Gemma 4 26B",
      "type": "llm",
      "publisher": "google",
      "architecture": "gemma",
      "quantization": {"name": "Q4_K_M", "bits_per_weight": 4},
      "format": "gguf",
      "size_bytes": 16000000000,
      "params_string": "26B-A4B",
      "max_context_length": 8192,
      "loaded_instances": [{"id": "inst-1"}],
      "capabilities": {"vision": true, "trained_for_tool_use": true,
                       "reasoning": {"allowed_options": ["off","on"], "default": "on"}}
    }
  ]
}`

func TestNormalizeV1ModelMapsAllFields(t *testing.T) {
	var payload dto.LMStudioV1Response
	if err := json.Unmarshal([]byte(fullV1Payload), &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	model := normalizeV1Model(payload.Models[0])

	if model.ID != "google/gemma-4-26b-a4b" {
		t.Errorf("ID = %q, want the v1 key", model.ID)
	}
	if model.Object != "model" {
		t.Errorf("Object = %q, want \"model\" (v1 omits it; v0 consumers expect a value)", model.Object)
	}
	if model.Arch != "gemma" {
		t.Errorf("Arch = %q, want \"gemma\" (from architecture)", model.Arch)
	}
	if model.CompatibilityType != "gguf" {
		t.Errorf("CompatibilityType = %q, want \"gguf\" (from format)", model.CompatibilityType)
	}
	if model.Quantization != "Q4_K_M" {
		t.Errorf("Quantization = %q, want \"Q4_K_M\" (from quantization.name)", model.Quantization)
	}
	if model.State != "loaded" {
		t.Errorf("State = %q, want \"loaded\" (loaded_instances is non-empty)", model.State)
	}
	if model.MaxContextLength != 8192 {
		t.Errorf("MaxContextLength = %d, want 8192", model.MaxContextLength)
	}
	if model.DisplayName != "Gemma 4 26B" {
		t.Errorf("DisplayName = %q, want \"Gemma 4 26B\"", model.DisplayName)
	}
	if model.SizeBytes != 16000000000 {
		t.Errorf("SizeBytes = %d, want 16000000000", model.SizeBytes)
	}
	if model.ParamsString != "26B-A4B" {
		t.Errorf("ParamsString = %q, want \"26B-A4B\"", model.ParamsString)
	}
	if !reflect.DeepEqual(model.LoadedInstanceIDs, []string{"inst-1"}) {
		t.Errorf("LoadedInstanceIDs = %v, want [inst-1]", model.LoadedInstanceIDs)
	}
}

func TestNormalizeV1ModelHandlesNulls(t *testing.T) {
	payloadJSON := `{"models":[{"key":"k","display_name":"d","type":"llm","publisher":"p",
	  "architecture":null,"quantization":null,"format":null,"size_bytes":0,
	  "params_string":null,"max_context_length":0,"loaded_instances":[],"capabilities":null}]}`

	var payload dto.LMStudioV1Response
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	model := normalizeV1Model(payload.Models[0])

	if model.Arch != "" {
		t.Errorf("Arch = %q, want empty for null architecture", model.Arch)
	}
	if model.Quantization != "" {
		t.Errorf("Quantization = %q, want empty for null quantization", model.Quantization)
	}
	if model.CompatibilityType != "" {
		t.Errorf("CompatibilityType = %q, want empty for null format", model.CompatibilityType)
	}
	if model.ParamsString != "" {
		t.Errorf("ParamsString = %q, want empty for null params_string", model.ParamsString)
	}
	if model.State != "not-loaded" {
		t.Errorf("State = %q, want \"not-loaded\" for empty loaded_instances", model.State)
	}
	if len(model.LoadedInstanceIDs) != 0 {
		t.Errorf("LoadedInstanceIDs = %v, want empty", model.LoadedInstanceIDs)
	}
}

func TestNormalizeV1Capabilities(t *testing.T) {
	tests := []struct {
		name  string
		model dto.LMStudioV1Model
		want  []string
	}{
		{
			name:  "embedding model ignores capabilities entirely",
			model: dto.LMStudioV1Model{Type: "embedding"},
			want:  []string{"embedding"},
		},
		{
			name:  "llm with no capabilities object",
			model: dto.LMStudioV1Model{Type: "llm"},
			want:  []string{"completion", "chat"},
		},
		{
			name: "llm with tools only",
			model: dto.LMStudioV1Model{Type: "llm",
				Capabilities: &dto.LMStudioV1Caps{TrainedForToolUse: true}},
			want: []string{"completion", "chat", "tools"},
		},
		{
			name: "llm with vision only",
			model: dto.LMStudioV1Model{Type: "llm",
				Capabilities: &dto.LMStudioV1Caps{Vision: true}},
			want: []string{"completion", "chat", "vision"},
		},
		{
			name: "llm with reasoning maps to thinking",
			model: dto.LMStudioV1Model{Type: "llm",
				Capabilities: &dto.LMStudioV1Caps{Reasoning: &dto.LMStudioV1Reasoning{Default: "on"}}},
			want: []string{"completion", "chat", "thinking"},
		},
		{
			name: "llm with everything",
			model: dto.LMStudioV1Model{Type: "llm",
				Capabilities: &dto.LMStudioV1Caps{Vision: true, TrainedForToolUse: true,
					Reasoning: &dto.LMStudioV1Reasoning{Default: "on"}}},
			want: []string{"completion", "chat", "tools", "vision", "thinking"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := normalizeV1Capabilities(test.model)
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("normalizeV1Capabilities() = %v, want %v", got, test.want)
			}
		})
	}
}
