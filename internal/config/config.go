package config

import (
	"flag"
	"fmt"
	"os"
)

type Config struct {
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string
	JWTSecret            string
}

func Load() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.RunAddress, "a", ":8080", "address and port to run server")
	flag.StringVar(&cfg.DatabaseURI, "d", "", "database connection URI")
	flag.StringVar(&cfg.AccrualSystemAddress, "r", "", "accrual system address")
	flag.Parse()

	if envAddr := os.Getenv("RUN_ADDRESS"); envAddr != "" {
		cfg.RunAddress = envAddr
	}
	if envDB := os.Getenv("DATABASE_URI"); envDB != "" {
		cfg.DatabaseURI = envDB
	}
	if envAccrual := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); envAccrual != "" {
		cfg.AccrualSystemAddress = envAccrual
	}
	if envSecret := os.Getenv("JWT_SECRET"); envSecret != "" {
		cfg.JWTSecret = envSecret
	}

	if cfg.DatabaseURI == "" {
		cfg.DatabaseURI = "postgres://postgres:postgres@localhost:5432/gophermart?sslmode=disable"
	}

	if cfg.JWTSecret == "" {
		fmt.Printf("JWT_SECRET is required (env JWT_SECRET or set explicitly)")
		os.Exit(1)
	}

	fmt.Printf("Config loaded: RUN_ADDRESS=%s, DATABASE_URI=%s, ACCRUAL_SYSTEM_ADDRESS=%s\n",
		cfg.RunAddress, cfg.DatabaseURI, cfg.AccrualSystemAddress)

	return cfg
}
