package cmd

import (
	"crypto/sha256"
	"fmt"
)

// generateChecksum calculates SHA256 hash of content
func generateChecksum(content []byte) string {
	hash := sha256.Sum256(content)
	return fmt.Sprintf("%x", hash)
}
