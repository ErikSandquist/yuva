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

// Key scopes keep an app's process key and a host backend's visitor key apart in KeyVisitorHash.
const (
	ScopeProcess    byte = 'p'
	ScopeVisitor    byte = 'k'
	ScopePersistent byte = 'c'
)

// KeyVisitorHash is the day's visitor for a key that stands in for the address: an app's process
// key or a host backend's own visitor key, as VisitorHash computes it for an address.
func KeyVisitorHash(salt []byte, property uuid.UUID, scope byte, key string) int64 {
	m := hmac.New(sha256.New, salt)
	m.Write(property[:])
	m.Write([]byte{scope})
	m.Write([]byte(key))
	return int64(binary.BigEndian.Uint64(m.Sum(nil)[:8]))
}

// PersistentHash is the visitor of a consented persistent id on every day: HMAC-SHA256 under the
// property's own key, which is random, kept sealed and never derived from a daily salt, over the
// property and the id, cut to 64 bits. The same id gives another value on every other property.
func PersistentHash(key []byte, property uuid.UUID, id string) int64 {
	m := hmac.New(sha256.New, key)
	m.Write(property[:])
	m.Write([]byte{ScopePersistent})
	m.Write([]byte(id))
	return int64(binary.BigEndian.Uint64(m.Sum(nil)[:8]))
}

// ValidPersistentID: 16 to 64 characters of the base64url alphabet, which a UUID also is.
func ValidPersistentID(id string) bool {
	if len(id) < 16 || len(id) > 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
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
