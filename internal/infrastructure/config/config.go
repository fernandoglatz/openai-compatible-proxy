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

		Redis struct {
			Address  string `yaml:"address"`
			Password string `yaml:"password"`
			Db       int    `yaml:"db"`

			TTL struct {
				Model time.Duration `yaml:"model"`
			} `yaml:"ttl"`
		} `yaml:"redis"`
	} `yaml:"data"`

	LMStudio struct {
		URL     string        `yaml:"url"`
		Timeout time.Duration `yaml:"timeout"`
		WOL     struct {
			Enabled          bool          `yaml:"enabled"`
			MacAddress       string        `yaml:"mac-address"`
			BroadcastAddress string        `yaml:"broadcast-address"`
			MaxRetries       int           `yaml:"max-retries"`
			RetryWait        time.Duration `yaml:"retry-wait"`
		} `yaml:"wol"`
	} `yaml:"lm-studio"`

	MQTT struct {
		Enabled  bool   `yaml:"enabled"`
		Broker   string `yaml:"broker"`
		ClientID string `yaml:"client-id"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Topic    string `yaml:"topic"`
		QOS      byte   `yaml:"qos"`
		Idle     struct {
			Timeout time.Duration `yaml:"timeout"`
			Message string        `yaml:"message"`
		} `yaml:"idle"`
	} `yaml:"mqtt"`

	Log struct {
		Level   string        `yaml:"level"`
		Format  format.Format `yaml:"format"`
		Colored bool          `yaml:"colored"`
	} `yaml:"log"`
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
