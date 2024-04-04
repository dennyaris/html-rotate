package util

import (
	"crypto/sha256"
	"encoding/hex"
)

func EncodeString(data string) string {
	hash := sha256.Sum256([]byte(data))
	newData := hex.EncodeToString([]byte(hash[:]))

	return newData
}
