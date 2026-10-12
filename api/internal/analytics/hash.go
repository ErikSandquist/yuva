package analytics

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"time"
	"uuid"
)

const SaltBytes = 32

// VisitorHash is the day's visitor: HMAC-SHA256 under the property's salt of the day over the
// property, the shortened address and the user agent, cut to 64 bits. Without the salt it cannot
// be recomputed.
func VisitorHash(salt []byte, property uuid.UUID, ip ShortIP, userAgent string) int64 {
	m := hmac.New(sha256.New, salt)
	m.Write(property[:])
	m.Write(ip.key())
	m.Write([]byte(userAgent))
	return int64(binary.BigEndian.Uint64(m.Sum(nil)[:8]))
}

// VisitHash names a visit by its visitor and first event.
func VisitHash(salt []byte, visitor int64, first time.Time) int64 {
	m := hmac.New(sha256.New, salt)
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], uint64(visitor))
	binary.BigEndian.PutUint64(b[8:], uint64(first.UnixMicro()))
	m.Write(b[:])
	return int64(binary.BigEndian.Uint64(m.Sum(nil)[:8]))
}

func NewSalt() []byte {
	b := make([]byte, SaltBytes)
	_, _ = rand.Read(b)
	return b
}
