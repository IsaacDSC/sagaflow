package cfg

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Task     Task
	Rollback Rollback
}

type Rollback struct {
	MaxRetry   int    `env:"ROLLBACK_MAX_RETRY" env-default:"3"`
	MaxTimeout string `env:"ROLLBACK_MAX_TIMEOUT" env-default:"1m"`
}

type Task struct {
	IntervalRollback     time.Duration `env:"TASK_INTERVAL_ROLLBACK" env-default:"1m"`
	IntervalRefreshRules time.Duration `env:"TASK_INTERVAL_REFRESH_RULES" env-default:"1m"`
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
