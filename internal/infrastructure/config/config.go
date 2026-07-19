package config

import (
	"context"
	"errors"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/constants"
	"fernandoglatz/openai-compatible-proxy/internal/core/common/utils/log"
	"fernandoglatz/openai-compatible-proxy/internal/infrastructure/config/format"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// SchedulerConfig controls the sticky-session request scheduler. MaxConcurrent is
// reserved; only a value of 1 (serialize) is currently honored.
//
// A queued request sends no bytes until it wins the slot, so an intermediary with a
// response timeout (CloudFront defaults to 30s) aborts it mid-wait. HeartbeatAfter and
// HeartbeatInterval keep such connections alive by emitting SSE comments while waiting;
// see heartbeatWriter. Zero HeartbeatInterval disables the heartbeat entirely.
type SchedulerConfig struct {
	Enabled           bool          `yaml:"enabled"`
	MaxConcurrent     int           `yaml:"max-concurrent"`
	IdleTimeout       time.Duration `yaml:"idle-timeout"`
	GatedPaths        []string      `yaml:"gated-paths"`
	HeartbeatAfter    time.Duration `yaml:"heartbeat-after"`
	HeartbeatInterval time.Duration `yaml:"heartbeat-interval"`
}

type Config struct {
	Server struct {
		Listening   string `yaml:"listening"`
		ContextPath string `yaml:"context-path"`
	} `yaml:"server"`

	Data struct {
		Mongo struct {
			Uri      string `yaml:"uri"`
			Database string `yaml:"database"`
		} `yaml:"mongo"`
	} `yaml:"data"`

	OpenAI struct {
		APIKeys []string `yaml:"api-keys"`
	} `yaml:"openai"`

	LMStudio struct {
		URL     string        `yaml:"url"`
		APIKey  string        `yaml:"api-key"`
		Timeout time.Duration `yaml:"timeout"`
		WOL     struct {
			Enabled          bool          `yaml:"enabled"`
			MacAddress       string        `yaml:"mac-address"`
			BroadcastAddress string        `yaml:"broadcast-address"`
			MaxRetries       int           `yaml:"max-retries"`
			RetryWait        time.Duration `yaml:"retry-wait"`
		} `yaml:"wol"`
	} `yaml:"lm-studio"`

	Log struct {
		Level   string        `yaml:"level"`
		Format  format.Format `yaml:"format"`
		Colored bool          `yaml:"colored"`
	} `yaml:"log"`

	Scheduler SchedulerConfig `yaml:"scheduler"`
}

type Tenant struct {
	Channel           string `yaml:"channel"`
	Credential        string `yaml:"credential"`
	PreferenceId      string `yaml:"preference-id"`
	CollectionPointId string `yaml:"collection-point-id"`
}

var ApplicationConfig Config

func LoadConfig(ctx context.Context) error {
	loadProfile(ctx)

	err := loadLocalConfig(ctx)
	if err != nil {
		return err
	}

	logConfig := ApplicationConfig.Log
	log.ReconfigureLogger(ctx, logConfig.Format, logConfig.Level, logConfig.Colored)

	return nil
}

func IsDevProfile() bool {
	profile := os.Getenv(constants.PROFILE)
	return constants.DEV_PROFILE == profile
}

func loadProfile(ctx context.Context) {
	profile := os.Getenv(constants.PROFILE)
	if len(profile) == constants.ZERO {
		profile = constants.DEV_PROFILE
		os.Setenv(constants.PROFILE, profile)
	}

	log.SetupLogger(profile) //after setup profile
	log.Info(ctx).Msg("Profile loaded: " + profile)
}

func loadLocalConfig(ctx context.Context) error {
	log.Info(ctx).Msg("Loading local config")

	data, err := os.ReadFile("conf/application.yml")
	if err != nil {
		return errors.New("Failed to read configuration file: " + err.Error())
	}

	err = yaml.Unmarshal(data, &ApplicationConfig)
	if err != nil {
		return errors.New("Failed to parse configuration file: " + err.Error())
	}

	log.Info(ctx).Msg("Loaded local config")

	return nil
}
