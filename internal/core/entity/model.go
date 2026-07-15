package entity

import (
	"time"
)

type Model struct {
	ID        string    `json:"id" bson:"id"`
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`

	Name              string `json:"name" bson:"name"`
	Object            string `json:"object" bson:"object"`
	Type              string `json:"type" bson:"type"`
	Publisher         string `json:"publisher" bson:"publisher"`
	Arch              string `json:"arch" bson:"arch"`
	CompatibilityType string `json:"compatibilityType" bson:"compatibilityType"`
	Quantization      string `json:"quantization" bson:"quantization"`
	State             string `json:"state" bson:"state"`
	MaxContextLength  int    `json:"maxContextLength" bson:"maxContextLength"`

	// Populated only by the LM Studio v1 sync. Documents written by the v0 path decode
	// these as zero values, which consumers treat as "unknown" and fall back on.
	DisplayName       string   `json:"displayName" bson:"displayName"`
	SizeBytes         int64    `json:"sizeBytes" bson:"sizeBytes"`
	ParamsString      string   `json:"paramsString" bson:"paramsString"`
	Capabilities      []string `json:"capabilities" bson:"capabilities"`
	LoadedInstanceIDs []string `json:"loadedInstanceIds" bson:"loadedInstanceIds"`
}
