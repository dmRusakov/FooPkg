package ramdomizer

import (
	"crypto/rand"
	"fmt"
	"time"
)

func GetRandomString(maxLength uint8) string {
	if maxLength == 0 {
		return ""
	}
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, maxLength)
	if _, err := rand.Read(bytes); err != nil {
		// fallback to timestamp-based string if crypto/rand fails
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		if len(timestamp) > int(maxLength) {
			return timestamp[:maxLength]
		}
		return timestamp
	}
	for i, b := range bytes {
		bytes[i] = charset[b%byte(len(charset))]
	}
	return string(bytes)
}
