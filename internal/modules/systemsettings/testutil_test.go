package systemsettings

import (
	"time"

	"matrix/internal/platform/config"
)

func DefaultTestConfig() *config.Config {
	cfg := config.Default()
	cfg.AI.DefaultModel.APIKey = "secret-key"
	cfg.AI.Security.ShellTimeout = 60 * time.Second
	cfg.Git.CloneTimeout = 300 * time.Second
	cfg.Worker.PollInterval = 2 * time.Second
	return cfg
}
