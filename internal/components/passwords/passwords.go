package passwords

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argon2Memory  uint32 = 19456
	argon2Time    uint32 = 2
	argon2Threads uint8  = 1
	saltLength           = 16
	keyLength     uint32 = 32
)

// Verified against when there is no real hash, so a failed sign-in costs the same as a real check.
const DummyHash = "$argon2id$v=19$m=19456,t=2,p=1$wo6At9qGGLcoVznCRNlAaw$TydLBTv8pfpcddqiVV/s/+3nCi63wE+cSVA0wv2GQgM"

func Hash(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, keyLength)
	return toPHCString(argon2Memory, argon2Time, argon2Threads, salt, key), nil
}

func Verify(password, encodedHash string) bool {
	memory, time, threads, salt, key, ok := fromPHCString(encodedHash)
	if !ok {
		return false
	}
	candidate := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(key)))
	return subtle.ConstantTimeCompare(candidate, key) == 1
}

func toPHCString(memory, time uint32, threads uint8, salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memory, time, threads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

func fromPHCString(encodedHash string) (memory, time uint32, threads uint8, salt, key []byte, ok bool) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return 0, 0, 0, nil, nil, false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return 0, 0, 0, nil, nil, false
	}
	if parts[3] != fmt.Sprintf("m=%d,t=%d,p=%d", memory, time, threads) || time < 1 || threads < 1 {
		return 0, 0, 0, nil, nil, false
	}
	salt, saltErr := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	key, keyErr := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if saltErr != nil || keyErr != nil || len(salt) == 0 || len(key) == 0 {
		return 0, 0, 0, nil, nil, false
	}
	return memory, time, threads, salt, key, true
}
