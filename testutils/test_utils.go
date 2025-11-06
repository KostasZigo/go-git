package testutils

import (
	"crypto/rand"
	"encoding/hex"
)

func RandomString(n int) string {
	bytes := make([]byte, n)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func RandomHash() string {
	return RandomString(20)
}
