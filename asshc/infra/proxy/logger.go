package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"assh/asshc/port"
	"assh/log"
)

type requestLogJSON struct {
	SessionID   string    `json:"session_id"`
	Timestamp   time.Time `json:"timestamp"`
	Protocol    string    `json:"protocol"`
	ClientAddr  string    `json:"client_addr"`
	TargetAddr  string    `json:"target_addr"`
	Action      string    `json:"action"`
	RuleMatched string    `json:"rule_matched"`
	BytesSent   int64     `json:"bytes_sent"`
	BytesRecv   int64     `json:"bytes_recv"`
	Duration    int64     `json:"duration_ms"`
	Error       string    `json:"error,omitempty"`
}

func toJSON(r *port.RequestLog) []byte {
	j := requestLogJSON{
		SessionID:   r.SessionID,
		Timestamp:   r.Timestamp,
		Protocol:    r.Protocol,
		ClientAddr:  r.ClientAddr,
		TargetAddr:  r.TargetAddr,
		Action:      r.Action,
		RuleMatched: r.RuleMatched,
		BytesSent:   r.BytesSent,
		BytesRecv:   r.BytesRecv,
		Duration:    r.Duration.Milliseconds(),
		Error:       r.Error,
	}
	data, err := json.Marshal(j)
	if err != nil {
		log.Errorf("proxy logger: marshal request log: %v", err)
		return nil
	}
	return data
}

// proxyLogger implements port.ProxyLogger with JSON line-delimited files.
type proxyLogger struct {
	mu          sync.Mutex
	dir         string
	file        *os.File
	buffer      []port.RequestLog
	batchSize   int
	closed      bool
	currentDate string
}

// NewProxyLogger creates a proxy logger writing JSON lines to the given directory.
// Log files are organized as: {logDir}/{date}/requests.jsonl
// Each line is a JSON object.
func NewProxyLogger(logDir string) (*proxyLogger, error) {
	if logDir == "" {
		return nil, fmt.Errorf("proxy logger: log directory cannot be empty")
	}
	absDir, err := filepath.Abs(logDir)
	if err != nil {
		return nil, fmt.Errorf("proxy logger: resolve log dir: %w", err)
	}
	return &proxyLogger{
		dir:       absDir,
		buffer:    make([]port.RequestLog, 0, 10),
		batchSize: 10,
	}, nil
}

func (l *proxyLogger) LogRequest(req *port.RequestLog) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return fmt.Errorf("proxy logger: already closed")
	}

	l.buffer = append(l.buffer, *req)
	if len(l.buffer) >= l.batchSize {
		return l.flush()
	}
	return nil
}

func (l *proxyLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}
	l.closed = true

	if err := l.flush(); err != nil {
		return err
	}
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func (l *proxyLogger) flush() error {
	if len(l.buffer) == 0 {
		return nil
	}

	if err := l.ensureFile(); err != nil {
		return err
	}

	for _, entry := range l.buffer {
		data := toJSON(&entry)
		if data == nil {
			continue
		}
		line := append(data, '\n')
		if _, err := l.file.Write(line); err != nil {
			return fmt.Errorf("proxy logger: write entry: %w", err)
		}
	}
	l.buffer = l.buffer[:0]
	return nil
}

func (l *proxyLogger) ensureFile() error {
	now := time.Now().UTC()
	dateStr := now.Format("2006-01-02")

	if l.file != nil && l.currentDate == dateStr {
		return nil
	}

	if l.file != nil {
		l.file.Close()
		l.file = nil
	}

	dir := filepath.Join(l.dir, dateStr)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("proxy logger: create log dir %s: %w", dir, err)
	}

	filePath := filepath.Join(dir, "requests.jsonl")
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("proxy logger: open log file %s: %w", filePath, err)
	}

	l.file = f
	l.currentDate = dateStr
	return nil
}
