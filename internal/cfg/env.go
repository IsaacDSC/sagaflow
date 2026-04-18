package cfg

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Task     Task
	Rollback Rollback
	Database Database
	Gqueue   Gqueue
}

// Gqueue holds optional settings for async saga steps via the gqueue pub/sub HTTP API.
type Gqueue struct {
	BaseURL              string `env:"GQUEUE_BASE_URL" env-required:"true"`
	BasicUser            string `env:"GQUEUE_BASIC_USER"`
	BasicPassword        string `env:"GQUEUE_BASIC_PASSWORD"`
	PublisherServiceName string `env:"GQUEUE_PUBLISHER_SERVICE_NAME" env-default:"sagaflow"`
}

type Rollback struct {
	MaxRetry   int    `env:"ROLLBACK_MAX_RETRY" env-default:"3"`
	MaxTimeout string `env:"ROLLBACK_MAX_TIMEOUT" env-default:"1m"`
}

type Task struct {
	IntervalRollback     time.Duration `env:"TASK_INTERVAL_ROLLBACK" env-default:"1m"`
	IntervalRefreshRules time.Duration `env:"TASK_INTERVAL_REFRESH_RULES" env-default:"1m"`
}

type Database struct {
	URL string `env:"DATABASE_URL"`
}

var cfg Config

func Load() error {
	err := cleanenv.ReadEnv(&cfg)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	return nil
}

func Get() Config {
	return cfg
}
