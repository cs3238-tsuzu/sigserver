package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/k0kubun/pp"
)

type InMemory struct {
	mut        sync.Mutex
	timeout    time.Duration
	registered map[string]*registeredData

	closed chan struct{}
}

func NewInMemory() *InMemory {
	return &InMemory{
		timeout:    1 * time.Minute,
		registered: make(map[string]*registeredData),

		closed: make(chan struct{}),
	}
}

func (in *InMemory) Register(key string) (string, error) {
	random := registeredData{
		token:     randomSecurePassword(),
		updatedAt: time.Now(),
	}

	in.mut.Lock()
	defer in.mut.Unlock()

	_, loaded := in.registered[key]

	if loaded {
		return "", ErrAlreadyRegistered
	}

	in.registered[key] = &random
	return random.token, nil
}

func (in *InMemory) Load(key, token string) error {
	in.mut.Lock()
	defer in.mut.Unlock()

	v, found := in.registered[key]

	if !found {
		return ErrUnknownKey
	}

	if v.IsTimeout(in.timeout) {
		delete(in.registered, key)

		return ErrUnknownKey
	}

	if v.token != token {
		return ErrDifferentToken
	}

	return nil
}

func (in *InMemory) Unregister(key, token string) error {
	in.mut.Lock()
	defer in.mut.Unlock()

	v, found := in.registered[key]

	if !found {
		return nil
	}

	if v.token != token {
		return ErrDifferentToken
	}

	delete(in.registered, key)

	return nil
}

func (in *InMemory) SetTimeout(d time.Duration) error {
	in.mut.Lock()

	in.timeout = d

	in.mut.Unlock()
	return nil
}

func (in *InMemory) Run() error {
	go func() {
		for {
			func() {
				in.mut.Lock()
				timeout := in.timeout
				in.mut.Unlock()
				t := time.NewTimer(1 * time.Second)

				select {
				case <-t.C:
				case <-in.closed:
					return
				}
				t.Stop()

				in.mut.Lock()
				defer in.mut.Unlock()
				del := make([]string, 0, 10)
				for k, v := range in.registered {
					fmt.Println(k, ": ", pp.Sprint(v))
					if v.IsTimeout(timeout) {
						del = append(del, k)
					}
				}

				for i := range del {
					delete(in.registered, del[i])
				}
			}()
		}
	}()

	return nil
}

func (in *InMemory) Stop() error {
	in.mut.Lock()
	defer in.mut.Unlock()

	close(in.closed)

	return nil
}

func (in *InMemory) Lock(key, token string) error {
	in.mut.Lock()
	defer in.mut.Unlock()

	v, found := in.registered[key]

	if !found || v.IsTimeout(in.timeout) {
		return ErrUnknownKey
	}

	log.Println(key, token, v)
	log.Println(len(token), len(v.token))
	if v.token != token {
		return ErrDifferentToken
	}

	v.counter++

	return nil
}
func (in *InMemory) Unlock(key string) error {
	in.mut.Lock()
	defer in.mut.Unlock()

	v, found := in.registered[key]

	if !found {
		return ErrUnknownKey
	}

	v.counter--
	v.updatedAt = time.Now()

	return nil
}
