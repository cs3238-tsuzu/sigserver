package main

import (
	"crypto/rand"
	"math/big"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" //_/><,.?@|{}[]`*:;!#$%&()=-^~"

func randomSecurePassword() string {
	b := make([]byte, 64)
	size := big.NewInt(int64(len(letterBytes)))
	for i := range b {
		c, _ := rand.Int(rand.Reader, size)
		b[i] = letterBytes[c.Int64()]
	}
	return string(b)
}
