package config

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"os"
)

type ServerConfig struct {
	Endpoint        string
	AccuralEndpoint string
	LogLevel        string
	DBConnection    string
	SHA256Key       string
}

func Parse() ServerConfig {
	var cfg ServerConfig
	flag.StringVar(&cfg.Endpoint, "a", "localhost:8081", "server host/port")
	flag.StringVar(&cfg.AccuralEndpoint, "r", "localhost:8080", "accural service host/port")
	//flag.StringVar(&cfg.DBConnection, "d", "postgres://postgres:admin@localhost:5432/postgres?sslmode=disable", "postgres database connection string")
	flag.StringVar(&cfg.DBConnection, "d", "", "postgres database connection string")
	flag.Parse()
	if res := os.Getenv("RUN_ADDRESS"); res != "" {
		cfg.Endpoint = res
	}
	if res := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); res != "" {
		cfg.AccuralEndpoint = res
	}
	if envLogLevel := os.Getenv("LOG_LEVEL"); envLogLevel != "" {
		cfg.LogLevel = envLogLevel
	}
	if res := os.Getenv("DATABASE_URI"); res != "" {
		cfg.DBConnection = res
	}
	if res := os.Getenv("AUTH_SECRET_KEY"); res != "" {
		cfg.SHA256Key = res
	} else {
		buffer := make([]byte, 12)
		_, err := rand.Read(buffer)
		if err != nil {
			panic(err)
		}
		cfg.SHA256Key = base64.StdEncoding.EncodeToString(buffer)
	}
	cfg.SHA256Key = "key"

	return cfg
}
