package api

import (
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/constants"
	"fernandoglatz/openai-compatible-proxy/internal/core/model/dto"
)

// Model states as reported to v0 consumers, which have no concept of loaded instances.
const (
	STATE_LOADED     = "loaded"
	STATE_NOT_LOADED = "not-loaded"
)

// Ollama's capability vocabulary, which entity.Model stores pre-normalized.
const (
	CAPABILITY_COMPLETION = "completion"
	CAPABILITY_CHAT       = "chat"
	CAPABILITY_TOOLS      = "tools"
	CAPABILITY_VISION     = "vision"
	CAPABILITY_THINKING   = "thinking"
	CAPABILITY_EMBEDDING  = "embedding"
)

// TYPE_EMBEDDING is the v1 model type for embedding models.
const TYPE_EMBEDDING = "embedding"

// OBJECT_MODEL is what v0 reports in its "object" field. v1 omits the field entirely,
// so the normalizer supplies it for consumers that still expect a value.
const OBJECT_MODEL = "model"

// normalizeV1Model converts a v1 model into the internal shape shared with v0, so
// callers of GetModels see one consistent type regardless of which upstream answered.
func normalizeV1Model(v1Model dto.LMStudioV1Model) dto.LMStudioModel {
	model := dto.LMStudioModel{
		ID:                v1Model.Key,
		Object:            OBJECT_MODEL,
		Type:              v1Model.Type,
		Publisher:         v1Model.Publisher,
		Arch:              derefString(v1Model.Architecture),
		CompatibilityType: derefString(v1Model.Format),
		State:             STATE_NOT_LOADED,
		MaxContextLength:  v1Model.MaxContextLength,
		DisplayName:       v1Model.DisplayName,
		SizeBytes:         v1Model.SizeBytes,
		ParamsString:      derefString(v1Model.ParamsString),
		Capabilities:      normalizeV1Capabilities(v1Model),
		LoadedInstanceIDs: make([]string, constants.ZERO),
	}

	if v1Model.Quantization != nil {
		model.Quantization = derefString(v1Model.Quantization.Name)
	}

	for _, instance := range v1Model.LoadedInstances {
		model.LoadedInstanceIDs = append(model.LoadedInstanceIDs, instance.ID)
	}

	if len(model.LoadedInstanceIDs) > constants.ZERO {
		model.State = STATE_LOADED
	}

	return model
}

// normalizeV1Capabilities maps v1's capabilities object onto Ollama's vocabulary.
// This is the only place the translation lives, so the Ollama controller stays a formatter.
func normalizeV1Capabilities(v1Model dto.LMStudioV1Model) []string {
	if v1Model.Type == TYPE_EMBEDDING {
		return []string{CAPABILITY_EMBEDDING}
	}

	capabilities := []string{CAPABILITY_COMPLETION, CAPABILITY_CHAT}

	if v1Model.Capabilities == nil {
		return capabilities
	}

	if v1Model.Capabilities.TrainedForToolUse {
		capabilities = append(capabilities, CAPABILITY_TOOLS)
	}

	if v1Model.Capabilities.Vision {
		capabilities = append(capabilities, CAPABILITY_VISION)
	}

	if v1Model.Capabilities.Reasoning != nil {
		capabilities = append(capabilities, CAPABILITY_THINKING)
	}

	return capabilities
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}
