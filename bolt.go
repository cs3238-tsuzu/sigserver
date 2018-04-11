package main

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/boltdb/bolt"
)

var boltBucketName = []byte("SIGNALING")

func NewBolt(path string) (*Bolt, error) {
	db, err := bolt.Open(path, 0664, nil)

	if err != nil {
		return nil, err
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(boltBucketName)

		return err
	}); err != nil {
		return nil, err
	}

	return &Bolt{
		db:      db,
		timeout: 1 * time.Minute,
		closed:  make(chan struct{}),
	}, nil
}

type Bolt struct {
	db *bolt.DB

	mut     sync.Mutex
	timeout time.Duration
	closed  chan struct{}
}

func (b *Bolt) do(f func(b *bolt.Bucket) error) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(boltBucketName)

		if b == nil {
			return ErrUnknownKey
		}

		return f(b)
	})
}

func (l *Bolt) Run() error {
	go func() {
		defer l.db.Close()
		for {
			l.mut.Lock()
			timeout := l.timeout
			l.mut.Unlock()
			t := time.NewTimer(timeout)

			select {
			case <-t.C:
			case <-l.closed:
				return
			}
			t.Stop()

			l.do(func(b *bolt.Bucket) error {
				keys := make([][]byte, 0, 10)
				b.ForEach(func(k []byte, v []byte) error {
					var rv registeredData
					if err := json.Unmarshal(v, &rv); err != nil {
						return nil
					}

					if rv.IsTimeout(timeout) {
						keys = append(keys, k)
					}

					return nil
				})

				for i := range keys {
					b.Delete(keys[i])
				}

				return nil
			})
		}
	}()

	return nil
}

func (b *Bolt) Stop() error {
	close(b.closed)

	return nil
}

func (b *Bolt) Register(k string) (res string, err error) {
	err = b.do(func(b *bolt.Bucket) error {
		if b.Get([]byte(k)) == nil {
			return ErrAlreadyRegistered
		}

		token := randomSecurePassword()

		j, _ := json.Marshal(
			registeredData{
				updatedAt: time.Now(),
				token:     token,
				counter:   0,
			},
		)

		res = token

		return b.Put(
			[]byte(k),
			j,
		)
	})

	return
}
func (b *Bolt) Load(key, token string) (err error) {
	err = b.do(func(bucket *bolt.Bucket) error {
		j := bucket.Get([]byte(key))

		var rv registeredData
		if err := json.Unmarshal(j, &rv); err != nil || rv.IsTimeout(b.timeout) {
			return ErrUnknownKey
		}

		if token != rv.token {
			return ErrDifferentToken
		}

		rv.updatedAt = time.Now()

		j, _ = json.Marshal(rv)

		return bucket.Put(
			[]byte(key),
			j,
		)
	})

	return
}

func (b *Bolt) Unregister(key, token string) (err error) {
	err = b.do(func(b *bolt.Bucket) error {
		j := b.Get([]byte(key))

		var rv registeredData
		if err := json.Unmarshal(j, &rv); err != nil {
			return ErrUnknownKey
		}

		if token != rv.token {
			return ErrDifferentToken
		}

		return b.Delete([]byte(key))
	})

	return
}

func (b *Bolt) SetTimeout(d time.Duration) error {
	b.mut.Lock()
	b.timeout = d
	b.mut.Unlock()

	return nil
}

func (b *Bolt) Lock(key, token string) (err error) {
	err = b.do(func(bucket *bolt.Bucket) error {
		j := bucket.Get([]byte(key))

		var rv registeredData
		if err := json.Unmarshal(j, &rv); err != nil || rv.IsTimeout(b.timeout) {
			return ErrUnknownKey
		}

		if token != rv.token {
			return ErrDifferentToken
		}

		rv.counter++

		j, _ = json.Marshal(rv)

		return bucket.Put(
			[]byte(key),
			j,
		)
	})

	return
}
func (b *Bolt) Unlock(key string) (err error) {
	err = b.do(func(b *bolt.Bucket) error {
		j := b.Get([]byte(key))

		var rv registeredData
		if err := json.Unmarshal(j, &rv); err != nil {
			return ErrUnknownKey
		}

		rv.counter--
		rv.updatedAt = time.Now()

		j, _ = json.Marshal(rv)

		return b.Put(
			[]byte(key),
			j,
		)
	})

	return
}
