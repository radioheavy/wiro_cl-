package task

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/wiro-ai/wiro-cli/internal/api"
)

const (
	wsURL = "wss://socket.wiro.ai/v1"
)

// Service manages run/detail/cancel/kill and watch operations.
type Service struct {
	apiClient *api.Client
}

func NewService(apiClient *api.Client) *Service {
	return &Service{apiClient: apiClient}
}

// WatchEvent streams progress details.
type WatchEvent struct {
	Source string
	Type   string
	Text   string
	Raw    map[string]interface{}
}

func isTerminal(status string) bool {
	switch status {
	case "task_postprocess_end", "task_cancel", "task_end", "task_error_full":
		return true
	default:
		return false
	}
}

func (s *Service) Run(ctx context.Context, owner, model string, values map[string][]api.MultipartValue, headers map[string]string) (api.RunResponse, error) {
	path := fmt.Sprintf("/Run/%s/%s", owner, model)
	var resp api.RunResponse
	if err := s.apiClient.PostMultipart(ctx, path, values, headers, &resp); err != nil {
		return api.RunResponse{}, err
	}
	if !resp.Result && len(resp.Errors) > 0 {
		return api.RunResponse{}, fmt.Errorf("run failed: %s", resp.Errors[0].Message)
	}
	return resp, nil
}

func (s *Service) Detail(ctx context.Context, idOrToken string, headers map[string]string) (api.TaskDetailResponse, error) {
	body := map[string]interface{}{}
	if looksLikeNumeric(idOrToken) {
		body["taskid"] = idOrToken
	} else {
		body["tasktoken"] = idOrToken
	}
	var resp api.TaskDetailResponse
	if err := s.apiClient.PostJSON(ctx, "/Task/Detail", body, headers, &resp); err != nil {
		return api.TaskDetailResponse{}, err
	}
	if !resp.Result && len(resp.Errors) > 0 {
		return resp, fmt.Errorf("task detail failed: %s", resp.Errors[0].Message)
	}
	return resp, nil
}

func (s *Service) Cancel(ctx context.Context, taskID string, headers map[string]string) (api.TaskDetailResponse, error) {
	var resp api.TaskDetailResponse
	if err := s.apiClient.PostJSON(ctx, "/Task/Cancel", map[string]interface{}{"taskid": taskID}, headers, &resp); err != nil {
		return api.TaskDetailResponse{}, err
	}
	if !resp.Result && len(resp.Errors) > 0 {
		return resp, fmt.Errorf("task cancel failed: %s", resp.Errors[0].Message)
	}
	return resp, nil
}

func (s *Service) Kill(ctx context.Context, taskID string, headers map[string]string) (api.TaskDetailResponse, error) {
	var resp api.TaskDetailResponse
	if err := s.apiClient.PostJSON(ctx, "/Task/Kill", map[string]interface{}{"taskid": taskID}, headers, &resp); err != nil {
		return api.TaskDetailResponse{}, err
	}
	if !resp.Result && len(resp.Errors) > 0 {
		return resp, fmt.Errorf("task kill failed: %s", resp.Errors[0].Message)
	}
	return resp, nil
}

// WatchTask combines websocket stream and polling fallback. It returns final task detail.
func (s *Service) WatchTask(ctx context.Context, taskToken string, headers map[string]string, onEvent func(WatchEvent)) (*api.Task, error) {
	if strings.TrimSpace(taskToken) == "" {
		return nil, errors.New("task token is required for watch")
	}
	finalTaskCh := make(chan *api.Task, 1)
	errCh := make(chan error, 2)
	var once sync.Once

	signalFinal := func(task *api.Task) {
		if task == nil {
			return
		}
		once.Do(func() {
			finalTaskCh <- task
		})
	}

	// Polling fallback (always on, low-frequency).
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				detail, err := s.Detail(ctx, taskToken, headers)
				if err != nil {
					errCh <- err
					continue
				}
				if len(detail.TaskList) == 0 {
					continue
				}
				task := detail.TaskList[0]
				if onEvent != nil {
					onEvent(WatchEvent{Source: "poll", Type: task.Status, Text: "polled status", Raw: map[string]interface{}{"status": task.Status}})
				}
				if isTerminal(task.Status) {
					signalFinal(&task)
					return
				}
			}
		}
	}()

	// Websocket stream
	go func() {
		conn, err := dialWS(ctx, wsURL)
		if err != nil {
			errCh <- fmt.Errorf("websocket connect failed (polling fallback active): %w", err)
			return
		}
		defer conn.Close()

		register := map[string]string{"type": "task_info", "tasktoken": taskToken}
		if err := conn.WriteJSON(register); err != nil {
			errCh <- fmt.Errorf("websocket register failed: %w", err)
			return
		}

		for {
			rawMsg, err := conn.ReadText()
			if err != nil {
				errCh <- fmt.Errorf("websocket read failed (polling fallback active): %w", err)
				return
			}
			msg := map[string]interface{}{}
			if err := json.Unmarshal(rawMsg, &msg); err != nil {
				continue
			}
			typeVal, _ := msg["type"].(string)
			text := ""
			if m, ok := msg["message"]; ok {
				b, _ := json.Marshal(m)
				text = string(b)
			}
			if onEvent != nil {
				onEvent(WatchEvent{Source: "ws", Type: typeVal, Text: text, Raw: msg})
			}
			if isTerminal(typeVal) {
				detail, err := s.Detail(ctx, taskToken, headers)
				if err == nil && len(detail.TaskList) > 0 {
					task := detail.TaskList[0]
					signalFinal(&task)
					return
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case task := <-finalTaskCh:
			return task, nil
		case err := <-errCh:
			if onEvent != nil {
				onEvent(WatchEvent{Source: "system", Type: "warning", Text: err.Error()})
			}
		}
	}
}

func looksLikeNumeric(v string) bool {
	if v == "" {
		return false
	}
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

type wsConn struct {
	conn   net.Conn
	reader *bufio.Reader
}

func dialWS(ctx context.Context, endpoint string) (*wsConn, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	dialer := &net.Dialer{}
	rawConn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, err
	}
	var conn net.Conn = rawConn
	if u.Scheme == "wss" {
		tlsConn := tls.Client(rawConn, &tls.Config{ServerName: strings.Split(u.Host, ":")[0]})
		if err := tlsConn.Handshake(); err != nil {
			rawConn.Close()
			return nil, err
		}
		conn = tlsConn
	}

	key, err := wsKey()
	if err != nil {
		conn.Close()
		return nil, err
	}
	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	hostHeader := u.Host
	if hostHeader == "" {
		hostHeader = host
	}

	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", path, hostHeader, key)
	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, err
	}

	br := bufio.NewReader(conn)
	statusLine, err := br.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}
	if !strings.Contains(statusLine, "101") {
		conn.Close()
		return nil, fmt.Errorf("websocket handshake failed: %s", strings.TrimSpace(statusLine))
	}

	headers := map[string]string{}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			headers[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
		}
	}

	expectedAccept := wsAccept(key)
	if got := headers["sec-websocket-accept"]; got != "" && got != expectedAccept {
		conn.Close()
		return nil, errors.New("websocket accept key mismatch")
	}

	return &wsConn{conn: conn, reader: br}, nil
}

func wsKey() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

func wsAccept(key string) string {
	h := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h[:])
}

func (w *wsConn) Close() error {
	if w == nil || w.conn == nil {
		return nil
	}
	return w.conn.Close()
}

func (w *wsConn) WriteJSON(v interface{}) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return w.writeFrame(0x1, payload)
}

func (w *wsConn) ReadText() ([]byte, error) {
	for {
		opcode, payload, err := w.readFrame()
		if err != nil {
			return nil, err
		}
		switch opcode {
		case 0x1:
			return payload, nil
		case 0x8:
			return nil, io.EOF
		case 0x9:
			if err := w.writeFrame(0xA, payload); err != nil {
				return nil, err
			}
		case 0xA:
			continue
		default:
			continue
		}
	}
}

func (w *wsConn) writeFrame(opcode byte, payload []byte) error {
	head := make([]byte, 0, 14)
	head = append(head, 0x80|(opcode&0x0F))

	payloadLen := len(payload)
	maskBit := byte(0x80)
	if payloadLen < 126 {
		head = append(head, maskBit|byte(payloadLen))
	} else if payloadLen <= 65535 {
		head = append(head, maskBit|126)
		buf := make([]byte, 2)
		binary.BigEndian.PutUint16(buf, uint16(payloadLen))
		head = append(head, buf...)
	} else {
		head = append(head, maskBit|127)
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(payloadLen))
		head = append(head, buf...)
	}

	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}
	head = append(head, mask...)

	masked := make([]byte, payloadLen)
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}

	if _, err := w.conn.Write(head); err != nil {
		return err
	}
	_, err := w.conn.Write(masked)
	return err
}

func (w *wsConn) readFrame() (byte, []byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(w.reader, header); err != nil {
		return 0, nil, err
	}
	opcode := header[0] & 0x0F
	masked := (header[1] & 0x80) != 0
	length := int64(header[1] & 0x7F)

	switch length {
	case 126:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(w.reader, ext); err != nil {
			return 0, nil, err
		}
		length = int64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(w.reader, ext); err != nil {
			return 0, nil, err
		}
		length = int64(binary.BigEndian.Uint64(ext))
	}

	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(w.reader, maskKey); err != nil {
			return 0, nil, err
		}
	}

	if length < 0 || length > 32*1024*1024 {
		return 0, nil, errors.New("websocket payload too large")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(w.reader, payload); err != nil {
		return 0, nil, err
	}

	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return opcode, payload, nil
}
