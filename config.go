package main

import (
	"time"

	"github.com/pkg/errors"
)

type StoreEnum int

const (
	StoreInMemory StoreEnum = iota
	StoreBolt
)

var globalConfig Config

type Config struct {
	Server ServerConfig `toml:"server"`

	Bolt *BoltConfig `toml:"bolt"`
}

type ServerConfig struct {
	Address                string             `toml:"address"` // "0.0.0.0:8080"(default)
	Store                  string             `toml:"store"`   // "in-memory"(default), "bolt"
	RegistrationTimeoutStr string             `toml:"registration_timeout"`
	MaxMessages            int32              `toml:"max_messages"`
	MaxLength              int32              `toml:"max_length"`
	CommunicateTimeoutStr  string             `toml:"communicate_timeout"`
	Certificate            *CertificateConfig `toml:"certificates"`

	storeEnum                               StoreEnum
	registrationTimeout, communicateTimeout time.Duration
}

type BoltConfig struct {
	File string `toml:"file"`
}

type CertificateConfig struct {
	KeyFile  string `toml:"keyFile"`
	CertFile string `toml:"certFile"`
}

func validateConfig(config *Config) error {
	if config.Server.Address == "" {
		config.Server.Address = "0.0.0.0:8080"
	}

	switch config.Server.Store {
	case "":
		config.Server.storeEnum = StoreInMemory
		config.Server.Store = "in-memory"

	case "in-memory":
		config.Server.storeEnum = StoreInMemory
	case "bolt":
		config.Server.storeEnum = StoreBolt

	default:
		return errors.New("invalid config.server.store")
	}

	if config.Server.MaxLength < 0 {
		return errors.New("invalid config.server.max_length")
	}
	if config.Server.MaxMessages < 0 {
		return errors.New("invalid config.server.max_messages")
	}

	var err error

	config.Server.communicateTimeout, err = time.ParseDuration(config.Server.CommunicateTimeoutStr)

	if config.Server.communicateTimeout < 0 {
		err = errors.New("must be >= 0")
	}

	if err != nil {
		return errors.Wrap(err, "invalid config.server.communicate_timeout")
	}

	config.Server.registrationTimeout, err = time.ParseDuration(config.Server.RegistrationTimeoutStr)

	if config.Server.registrationTimeout < 0 {
		err = errors.New("must be  >= 0")
	}

	if err != nil {
		return errors.Wrap(err, "invalid config.server.registration_timeout")
	}

	return nil
}
