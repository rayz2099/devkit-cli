package codews

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const wsMagic = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// wsHub 把 watch 事件推给已升级的连接。客户端消息没有业务含义。
type wsHub struct {
	mu    sync.Mutex
	conns []net.Conn
}

func newHub() *wsHub { return &wsHub{} }

func (h *wsHub) add(c net.Conn) {
	h.mu.Lock()
	h.conns = append(h.conns, c)
	h.mu.Unlock()
}

func (h *wsHub) remove(c net.Conn) {
	h.mu.Lock()
	next := h.conns[:0]
	for _, item := range h.conns {
		if item != c {
			next = append(next, item)
		}
	}
	h.conns = next
	h.mu.Unlock()
}

func (h *wsHub) publish(msg []byte) {
	frame := wsFrame(0x1, msg)
	h.mu.Lock()
	defer h.mu.Unlock()
	next := h.conns[:0]
	for _, c := range h.conns {
		_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := c.Write(frame); err != nil {
			_ = c.Close()
			continue
		}
		next = append(next, c)
	}
	h.conns = next
}

func (h *wsHub) upgrade(w http.ResponseWriter, r *http.Request) {
	if !headerToken(r.Header, "Connection", "upgrade") || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		writeText(w, http.StatusUpgradeRequired, "websocket upgrade required")
		return
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		writeText(w, http.StatusUpgradeRequired, "websocket upgrade required")
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		writeText(w, http.StatusUpgradeRequired, "websocket upgrade required")
		return
	}
	conn, buf, err := hj.Hijack()
	if err != nil {
		return
	}
	sum := sha1.Sum([]byte(key + wsMagic))
	accept := base64.StdEncoding.EncodeToString(sum[:])
	_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	_, _ = buf.WriteString("Upgrade: websocket\r\n")
	_, _ = buf.WriteString("Connection: Upgrade\r\n")
	_, _ = buf.WriteString("Sec-WebSocket-Accept: " + accept + "\r\n\r\n")
	_ = buf.Flush()
	h.add(conn)
	go h.readLoop(conn, buf.Reader)
}

func (h *wsHub) readLoop(conn net.Conn, buf *bufio.Reader) {
	defer func() {
		h.remove(conn)
		_ = conn.Close()
	}()
	if buf == nil {
		buf = bufio.NewReader(conn)
	}
	for {
		hdr := make([]byte, 2)
		if _, err := io.ReadFull(buf, hdr); err != nil {
			return
		}
		opcode := hdr[0] & 0x0f
		masked := hdr[1]&0x80 != 0
		ln := int(hdr[1] & 0x7f)
		switch ln {
		case 126:
			ext := make([]byte, 2)
			if _, err := io.ReadFull(buf, ext); err != nil {
				return
			}
			ln = int(ext[0])<<8 | int(ext[1])
		case 127:
			return
		}
		var mask [4]byte
		if masked {
			if _, err := io.ReadFull(buf, mask[:]); err != nil {
				return
			}
		}
		payload := make([]byte, ln)
		if ln > 0 {
			if _, err := io.ReadFull(buf, payload); err != nil {
				return
			}
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}
		switch opcode {
		case 0x8:
			return
		case 0x9:
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, _ = conn.Write(wsFrame(0xA, payload))
		}
	}
}

func wsFrame(opcode byte, payload []byte) []byte {
	n := len(payload)
	hdr := []byte{0x80 | opcode}
	switch {
	case n < 126:
		hdr = append(hdr, byte(n))
	case n < 65536:
		hdr = append(hdr, 126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, 127, 0, 0, 0, 0, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
	return append(hdr, payload...)
}

func headerToken(h http.Header, key, token string) bool {
	for _, part := range strings.Split(h.Get(key), ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}
