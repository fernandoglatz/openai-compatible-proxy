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
}
