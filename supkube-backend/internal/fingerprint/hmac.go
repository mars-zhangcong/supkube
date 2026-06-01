// HMAC helpers — extracted so writer.go and validator.go share the EXACT
// same input construction. A one-byte drift here (e.g. trailing newline)
// would produce silent false-negatives at verify time.
package fingerprint

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// computeHMAC returns the lowercase-hex HMAC-SHA256 of the canonical
// "{sourceClusterID}|{backupName}|{tarballSHA256}|{createdAt}" string
// keyed by `secret`. The pipe separator + field order is the contract
// — do not reformat.
func computeHMAC(secret []byte, fp *Fingerprint) string {
	mac := hmac.New(sha256.New, secret)
	// Building via strings.Builder rather than fmt.Sprintf to be 100%
	// sure no locale-sensitive formatter ever sneaks a different rune in.
	var b strings.Builder
	b.WriteString(fp.SourceClusterID)
	b.WriteByte('|')
	b.WriteString(fp.BackupName)
	b.WriteByte('|')
	b.WriteString(fp.TarballSHA256)
	b.WriteByte('|')
	b.WriteString(fp.CreatedAt)
	mac.Write([]byte(b.String()))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyHMAC returns true when the recomputed HMAC matches the one
// recorded in fp.HMACSHA256, using constant-time compare to defeat the
// (theoretical) timing oracle. Both sides are hex-encoded so length is
// fixed at 64; we still gate on length to be defensive against truncation.
func verifyHMAC(secret []byte, fp *Fingerprint) bool {
	if len(fp.HMACSHA256) != 64 {
		return false
	}
	want := computeHMAC(secret, fp)
	return hmac.Equal([]byte(want), []byte(fp.HMACSHA256))
}
