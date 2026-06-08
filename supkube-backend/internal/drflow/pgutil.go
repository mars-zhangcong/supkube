package drflow

import (
	"crypto/md5" //nolint:gosec
	"fmt"
	"io"
	"net"
)

// pgPasswordMessage builds a PG3 PasswordMessage packet.
func pgPasswordMessage(password string) []byte {
	body := []byte(password)
	body = append(body, 0) // null terminator
	msg := make([]byte, 1+4+len(body))
	msg[0] = 'p'
	total := uint32(4 + len(body))
	msg[1], msg[2], msg[3], msg[4] = byte(total>>24), byte(total>>16), byte(total>>8), byte(total)
	copy(msg[5:], body)
	return msg
}

// pgMD5Password computes md5(md5(password+user)+salt) in hex, prefixed "md5".
func pgMD5Password(user, password string, salt []byte) string {
	inner := md5.Sum([]byte(password + user))                           //nolint:gosec
	outer := md5.Sum(append([]byte(fmt.Sprintf("%x", inner)), salt...)) //nolint:gosec
	return "md5" + fmt.Sprintf("%x", outer)
}

// pgErrorMessage extracts a human-readable message from a PG3 ErrorResponse body.
// Fields are 'M' (message), 'D' (detail), 'H' (hint); we return the first 'M'.
func pgErrorMessage(body []byte) string {
	i := 0
	for i < len(body) {
		code := body[i]
		i++
		start := i
		for i < len(body) && body[i] != 0 {
			i++
		}
		if code == 'M' {
			return string(body[start:i])
		}
		i++ // skip null
	}
	return "(unknown error)"
}

// readFull reads exactly len(buf) bytes from conn.
func readFull(conn net.Conn, buf []byte) (int, error) {
	return io.ReadFull(conn, buf)
}
