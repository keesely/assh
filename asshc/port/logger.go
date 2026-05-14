package port

import "time"

// ProxyLogger defines the transparent proxy logging interface.
// Logs every proxy request/response for auditing and debugging.
type ProxyLogger interface {
	// LogRequest records a completed proxy request.
	LogRequest(req *RequestLog) error

	// Close flushes and closes the logger.
	Close() error
}

// RequestLog contains the full record of a proxy request.
type RequestLog struct {
	SessionID   string        `json:"session_id"`
	Timestamp   time.Time     `json:"timestamp"`
	Protocol    string        `json:"protocol"`     // "socks5", "http", "direct"
	ClientAddr  string        `json:"client_addr"`
	TargetAddr  string        `json:"target_addr"`  // "host:port"
	Action      string        `json:"action"`       // "proxy" or "direct"
	RuleMatched string        `json:"rule_matched"` // matched rule pattern, if any
	BytesSent   int64         `json:"bytes_sent"`
	BytesRecv   int64         `json:"bytes_recv"`
	Duration    time.Duration `json:"duration_ms"`  // in milliseconds
	Error       string        `json:"error,omitempty"`
}
