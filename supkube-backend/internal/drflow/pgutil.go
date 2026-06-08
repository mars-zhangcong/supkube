package drflow

import (
	"context"
	"crypto/md5" //nolint:gosec
	"fmt"
	"io"
	"net"
	"time"
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

// pgDial dials addr, performs the PG3 handshake, and returns the open connection.
// Caller is responsible for closing the connection.
func pgDial(ctx context.Context, addr, user, pass, dbName string) (net.Conn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("TCP dial %s: %w", addr, err)
	}
	conn.SetDeadline(time.Now().Add(15 * time.Second)) //nolint:errcheck

	startup := pgStartupMessage(user, dbName)
	if _, err := conn.Write(startup); err != nil {
		conn.Close() //nolint:errcheck
		return nil, fmt.Errorf("write startup: %w", err)
	}
	if err := pgHandshake(conn, user, pass); err != nil {
		conn.Close() //nolint:errcheck
		return nil, err
	}
	return conn, nil
}

// pgQueryMessage builds a PG3 Simple Query 'Q' message.
func pgQueryMessage(sql string) []byte {
	body := append([]byte(sql), 0) // null-terminate
	msg := make([]byte, 1+4+len(body))
	msg[0] = 'Q'
	total := uint32(4 + len(body))
	msg[1], msg[2], msg[3], msg[4] = byte(total>>24), byte(total>>16), byte(total>>8), byte(total)
	copy(msg[5:], body)
	return msg
}

// pgSimpleQuery connects, authenticates, sends sql, and returns the first column
// of the first DataRow. Returns an error if no row is returned.
func pgSimpleQuery(ctx context.Context, addr, user, pass, dbName, sql string) (string, error) {
	conn, err := pgDial(ctx, addr, user, pass, dbName)
	if err != nil {
		return "", err
	}
	defer conn.Close() //nolint:errcheck

	if _, err := conn.Write(pgQueryMessage(sql)); err != nil {
		return "", fmt.Errorf("send query: %w", err)
	}
	return pgReadFirstColumn(conn)
}

// pgExec connects, authenticates, sends sql, and waits for CommandComplete.
// Intended for DML statements (INSERT, UPDATE, DELETE) that return no rows.
func pgExec(ctx context.Context, addr, user, pass, dbName, sql string) error {
	conn, err := pgDial(ctx, addr, user, pass, dbName)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck

	if _, err := conn.Write(pgQueryMessage(sql)); err != nil {
		return fmt.Errorf("send exec: %w", err)
	}

	// Read until ReadyForQuery, checking for errors
	buf := make([]byte, 4096)
	for {
		if _, err := readFull(conn, buf[:1]); err != nil {
			return fmt.Errorf("read msg type: %w", err)
		}
		msgType := buf[0]
		if _, err := readFull(conn, buf[:4]); err != nil {
			return fmt.Errorf("read msg len: %w", err)
		}
		msgLen := int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
		body := make([]byte, msgLen-4)
		if len(body) > 0 {
			if _, err := readFull(conn, body); err != nil {
				return fmt.Errorf("read msg body: %w", err)
			}
		}
		switch msgType {
		case 'C': // CommandComplete — success
			continue
		case 'Z': // ReadyForQuery — done
			return nil
		case 'E': // ErrorResponse
			return fmt.Errorf("PG exec error: %s", pgErrorMessage(body))
		case 'I': // EmptyQueryResponse
			continue
		default:
			// Ignore any other message types (RowDescription 'T', DataRow 'D', etc.)
			continue
		}
	}
}

// pgReadFirstColumn reads DataRow messages from conn after a Simple Query has been
// sent, returning the first column of the first row as a string.
func pgReadFirstColumn(conn net.Conn) (string, error) {
	buf := make([]byte, 4096)
	result := ""
	found := false
	for {
		if _, err := readFull(conn, buf[:1]); err != nil {
			return "", fmt.Errorf("read msg type: %w", err)
		}
		msgType := buf[0]
		if _, err := readFull(conn, buf[:4]); err != nil {
			return "", fmt.Errorf("read msg len: %w", err)
		}
		msgLen := int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
		body := make([]byte, msgLen-4)
		if len(body) > 0 {
			if _, err := readFull(conn, body); err != nil {
				return "", fmt.Errorf("read msg body: %w", err)
			}
		}
		switch msgType {
		case 'T': // RowDescription — skip metadata
			continue
		case 'D': // DataRow — parse first column of first row
			if !found && len(body) >= 2 {
				// DataRow format: int16 num_cols, then for each col: int32 colLen (-1=null), colData
				// colCount := int(body[0])<<8 | int(body[1])
				if len(body) >= 6 {
					colLen := int(int32(body[2])<<24 | int32(body[3])<<16 | int32(body[4])<<8 | int32(body[5]))
					if colLen >= 0 && 6+colLen <= len(body) {
						result = string(body[6 : 6+colLen])
						found = true
					}
				}
			}
		case 'C': // CommandComplete
			continue
		case 'Z': // ReadyForQuery — done
			if !found {
				return "", fmt.Errorf("query returned no rows")
			}
			return result, nil
		case 'E': // ErrorResponse
			return "", fmt.Errorf("PG query error: %s", pgErrorMessage(body))
		case 'I': // EmptyQueryResponse
			continue
		default:
			continue
		}
	}
}
