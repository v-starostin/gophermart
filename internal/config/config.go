package config

import (
	"flag"

	"github.com/caarlos0/env/v10"
)

type Config struct {
	Address     string `env:"RUN_ADDRESS"`
	DatabaseURI string `env:"DATABASE_URI"`
	Secret      string `env:"SECRET"`
}

func New() (*Config, error) {
	flags := parseFlags()

	var cfg *Config
	err := env.Parse(cfg)
	if err != nil {
		return nil, err
	}

	if cfg.Address == "" {
		cfg.Address = flags.Address
	}
	if cfg.DatabaseURI == "" {
		cfg.DatabaseURI = flags.DatabaseURI
	}

	return cfg, nil
}

// todo: set default values
func parseFlags() *Config {
	address := flag.String("a", "", "HTTP server address")
	dbURI := flag.String("d", "", "DB URI")
	return &Config{Address: *address, DatabaseURI: *dbURI}
}
