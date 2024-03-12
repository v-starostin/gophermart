package config

import (
	"flag"

	"github.com/caarlos0/env/v10"
)

type Config struct {
	Address        string `env:"RUN_ADDRESS"`
	DatabaseURI    string `env:"DATABASE_URI"`
	Secret         string `env:"SECRET" envDefault:"key"`
	AccrualAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
}

func New() (*Config, error) {
	flags := parseFlags()

	cfg := &Config{}
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
	if cfg.AccrualAddress == "" {
		cfg.AccrualAddress = flags.AccrualAddress
	}

	return cfg, nil
}

func parseFlags() *Config {
	address := flag.String("a", "", "HTTP server address")
	dbURI := flag.String("d", "", "DB URI")
	accrualAddress := flag.String("r", "", "Accrual System address")
	flag.Parse()

	return &Config{Address: *address, DatabaseURI: *dbURI, AccrualAddress: *accrualAddress}
}
