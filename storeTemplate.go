package main

import (
	"time"
)

type KeyValueStore interface {
	Register(key string) (string, error)
	Load(key, token string) error

	Unregister(key, token string) error

	SetTimeout(time.Duration) error

	Lock(key, token string) error
	Unlock(key string) error

	Run() error
	Stop() error
}

type registeredData struct {
	UpdatedAt time.Time
	Token     string
	Counter   int
}

func (rd *registeredData) IsTimeout(d time.Duration) bool {
	return rd.Counter == 0 && time.Now().Sub(rd.UpdatedAt) > d
}
