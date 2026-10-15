package td

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

const maxFrame = 64 << 20

type queryIn struct {
	Host       string
	Token      string
	BFF        bool
	Prefix     string
	ProjectID  int
	EventModel int
	QP         json.RawMessage
	Timeout    time.Duration
	OnProgress func(string) error
}

type waitIn struct {
	URL        string
	Frame      string
	Kill       string
	ID         string
	Timeout    time.Duration
	OnProgress func(string) error
}

// runQuery 把 QP 交给 websocket. 事件分析和属性分析的计算不在 REST 上返回.
func runQuery(in queryIn) (json.RawMessage, error) {
	id, err := requestID(in.Prefix)
	if err != nil {
		return nil, err
	}
	frame, err := buildFrame(queryPayload{
		RequestID:    id,
		ProjectID:    in.ProjectID,
		EventModel:   in.EventModel,
		QP:           string(in.QP),
		SearchSource: "model_search",
		QuerySource:  "module",
	})
	if err != nil {
		return nil, err
	}
	kill, err := killFrame(id)
	if err != nil {
		return nil, err
	}
	addr, err := wsURL(in.Host, in.Token, in.BFF)
	if err != nil {
		return nil, err
	}
	return waitResult(waitIn{
		URL:        addr,
		Frame:      frame,
		Kill:       kill,
		ID:         id,
		Timeout:    in.Timeout,
		OnProgress: in.OnProgress,
	})
}

func waitResult(in waitIn) (json.RawMessage, error) {
	deadline := time.Now().Add(in.Timeout)
	conn, reader, err := dialWS(in.URL, deadline)
	if err != nil {
		return nil, fmt.Errorf("websocket error")
	}
	defer conn.Close()
	if err := writeText(conn, in.Frame); err != nil {
		return nil, fmt.Errorf("websocket error")
	}
	for {
		opcode, payload, err := readFrame(reader)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				sendKill(conn, in.Kill)
				return nil, fmt.Errorf("query timeout %dms", in.Timeout.Milliseconds())
			}
			return nil, fmt.Errorf("websocket closed")
		}
		switch opcode {
		case 0x8:
			return nil, fmt.Errorf("websocket closed")
		case 0x9:
			if err := writeFrame(conn, 0xA, payload); err != nil {
				return nil, fmt.Errorf("websocket error")
			}
			continue
		case 0xA:
			continue
		case 0x1:
		default:
			continue
		}
		msg := parseWS(string(payload))
		switch msg.Kind {
		case wsHeartbeat:
			if err := writeText(conn, `["heartbeat", ""]`); err != nil {
				return nil, fmt.Errorf("websocket error")
			}
		case wsFailed:
			if msg.ID == "" || msg.ID == in.ID {
				return nil, fmt.Errorf("%s", msg.Message)
			}
		case wsProgress:
			if msg.ID == in.ID && in.OnProgress != nil {
				line := strings.TrimSpace(msg.Progress.String() + " " + msg.Hint)
				if err := in.OnProgress(line); err != nil {
					return nil, err
				}
			}
		case wsDone:
			if msg.ID == in.ID {
				return msg.Result, nil
			}
		}
	}
}

func sendKill(conn net.Conn, kill string) {
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	_ = writeText(conn, kill)
}

func dialWS(raw string, deadline time.Time) (net.Conn, *bufio.Reader, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, nil, err
	}
	host := u.Host
	if u.Port() == "" {
		port := "80"
		if u.Scheme == "wss" {
			port = "443"
		}
		host = net.JoinHostPort(u.Hostname(), port)
	}
	conn, err := (&net.Dialer{Deadline: deadline}).Dial("tcp", host)
	if err != nil {
		return nil, nil, err
	}
	if err := conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return nil, nil, err
	}
	if u.Scheme == "wss" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: u.Hostname()})
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return nil, nil, err
		}
		conn = tlsConn
	}
	key, err := wsKey()
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	req := fmt.Sprintf(
		"GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		u.RequestURI(), u.Host, key,
	)
	if _, err := io.WriteString(conn, req); err != nil {
		conn.Close()
		return nil, nil, err
	}
	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	if !strings.Contains(status, " 101 ") {
		conn.Close()
		return nil, nil, fmt.Errorf("status %s", strings.TrimSpace(status))
	}
	accept := ""
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, nil, err
		}
		trim := strings.TrimRight(line, "\r\n")
		if trim == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(trim), "sec-websocket-accept:") {
			accept = strings.TrimSpace(trim[len("sec-websocket-accept:"):])
		}
	}
	if accept != wsAccept(key) {
		conn.Close()
		return nil, nil, fmt.Errorf("bad accept")
	}
	return conn, reader, nil
}

func wsKey() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf[:]), nil
}

func wsAccept(key string) string {
	sum := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func writeText(conn net.Conn, text string) error {
	return writeFrame(conn, 0x1, []byte(text))
}

func writeFrame(w io.Writer, opcode byte, payload []byte) error {
	n := len(payload)
	hdr := []byte{0x80 | opcode}
	switch {
	case n < 126:
		hdr = append(hdr, 0x80|byte(n))
	case n < 65536:
		hdr = append(hdr, 0x80|126, byte(n>>8), byte(n))
	default:
		ext := make([]byte, 8)
		binary.BigEndian.PutUint64(ext, uint64(n))
		hdr = append(hdr, 0x80|127)
		hdr = append(hdr, ext...)
	}
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return err
	}
	masked := make([]byte, n)
	for i := range masked {
		masked[i] = payload[i] ^ mask[i%4]
	}
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	if _, err := w.Write(mask[:]); err != nil {
		return err
	}
	_, err := w.Write(masked)
	return err
}

func readFrame(r *bufio.Reader) (byte, []byte, error) {
	var acc []byte
	var opcode byte
	first := true
	for {
		hdr := make([]byte, 2)
		if _, err := io.ReadFull(r, hdr); err != nil {
			return 0, nil, err
		}
		op := hdr[0] & 0x0f
		fin := hdr[0]&0x80 != 0
		if first {
			opcode = op
			first = false
		}
		masked := hdr[1]&0x80 != 0
		ln := uint64(hdr[1] & 0x7f)
		switch ln {
		case 126:
			ext := make([]byte, 2)
			if _, err := io.ReadFull(r, ext); err != nil {
				return 0, nil, err
			}
			ln = uint64(binary.BigEndian.Uint16(ext))
		case 127:
			ext := make([]byte, 8)
			if _, err := io.ReadFull(r, ext); err != nil {
				return 0, nil, err
			}
			ln = binary.BigEndian.Uint64(ext)
		}
		if ln > maxFrame || uint64(len(acc))+ln > maxFrame {
			return 0, nil, fmt.Errorf("websocket frame too large")
		}
		var mask [4]byte
		if masked {
			if _, err := io.ReadFull(r, mask[:]); err != nil {
				return 0, nil, err
			}
		}
		payload := make([]byte, ln)
		if _, err := io.ReadFull(r, payload); err != nil {
			return 0, nil, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}
		acc = append(acc, payload...)
		if fin {
			return opcode, acc, nil
		}
	}
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
