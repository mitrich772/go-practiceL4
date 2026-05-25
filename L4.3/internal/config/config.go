package config

import (
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config хранит настройки сервиса из YAML-файла.
type Config struct {
	Env        string     `yaml:"env" env-default:"local"`
	Storage    Storage    `yaml:"storage"`
	HTTPServer HTTPServer `yaml:"http_server"`
	Workers    Workers    `yaml:"workers"`
}

// Storage содержит параметры подключения к PostgreSQL.
type Storage struct {
	Host            string        `yaml:"host" env-required:"true"`
	Port            int           `yaml:"port" env-default:"5432"`
	User            string        `yaml:"user" env-required:"true"`
	Password        string        `yaml:"password" env-required:"true"`
	DBName          string        `yaml:"dbname" env-required:"true"`
	SSLMode         string        `yaml:"sslmode" env-default:"disable"`
	MaxOpenConns    int           `yaml:"max_open_conns" env-default:"10"`
	MaxIdleConns    int           `yaml:"max_idle_conns" env-default:"5"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env-default:"30m"`
}

// HTTPServer содержит параметры HTTP-сервера.
type HTTPServer struct {
	Address     string        `yaml:"address" env-default:"localhost:1235"`
	Timeout     time.Duration `yaml:"timeout" env-default:"4s"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s"`
}

// Workers содержит настройки фоновых горутин.
type Workers struct {
	ArchiveEvery   time.Duration `yaml:"archive_every" env-default:"1m"`
	ReminderBuffer int           `yaml:"reminder_buffer" env-default:"100"`
	LogBuffer      int           `yaml:"log_buffer" env-default:"100"`
}

// MustLoad читает конфиг из файла или завершает приложение при ошибке.
func MustLoad(configPath string) *Config {
	if configPath == "" {
		configPath = os.Getenv("CONFIG_PATH")
	}
	if configPath == "" {
		configPath = "config/local.yaml"
	}
	if _, err := os.Stat(configPath); err != nil {
		log.Fatalf("config file does not exist: %s", configPath)
	}

	var cfg Config
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("cannot read config: %s", err)
	}

	return &cfg
}
