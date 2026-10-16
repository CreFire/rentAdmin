package utils

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// NewID returns prefix + UTC timestamp + 8 random hex chars, e.g. "ord_20260101120000a1b2c3d4".
// Sortable by creation time and collision-safe within the same second.
func NewID(prefix string) string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return prefix + time.Now().UTC().Format("20060102150405") + hex.EncodeToString(buf)
}
