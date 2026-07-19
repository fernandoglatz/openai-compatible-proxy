package entity

import (
	"time"
)

type Model struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	Name              string `json:"name"`
	Object            string `json:"object"`
	Type              string `json:"type"`
	Publisher         string `json:"publisher"`
	Arch              string `json:"arch"`
	CompatibilityType string `json:"compatibilityType"`
	Quantization      string `json:"quantization"`
	State             string `json:"state"`
	MaxContextLength  int    `json:"maxContextLength"`

	// Populated only by the LM Studio v1 sync. Rows written by the v0 path leave these
	// at their column defaults, which consumers treat as "unknown" and fall back on.
	DisplayName       string   `json:"displayName"`
	SizeBytes         int64    `json:"sizeBytes"`
	ParamsString      string   `json:"paramsString"`
	Capabilities      []string `json:"capabilities"`
	LoadedInstanceIDs []string `json:"loadedInstanceIds"`
}
