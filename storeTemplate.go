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
	updatedAt time.Time
	token     string
	counter   int
}

func (rd *registeredData) IsTimeout(d time.Duration) bool {
	return rd.counter == 0 && time.Now().Sub(rd.updatedAt) > d
}
