package config

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestSchedulerConfigParses(t *testing.T) {
	raw := `
scheduler:
  enabled: true
  max-concurrent: 1
  idle-timeout: 10s
  gated-paths:
    - /v1/chat/completions
    - /v1/completions
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	s := cfg.Scheduler
	if !s.Enabled {
		t.Errorf("Enabled = false, want true")
	}
	if s.IdleTimeout != 10*time.Second {
		t.Errorf("IdleTimeout = %v, want 10s", s.IdleTimeout)
	}
	if len(s.GatedPaths) != 2 || s.GatedPaths[0] != "/v1/chat/completions" {
		t.Errorf("GatedPaths = %v, unexpected", s.GatedPaths)
	}
}
