package proxy

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	"assh/asshc/port"
	"assh/log"
)

// ReconnectConfig defines the auto-reconnection policy for tunnel manager.
type ReconnectConfig struct {
	MaxRetries int           // 0 = unlimited retries
	Interval   time.Duration // wait between retries (default 5s)
	Backoff    time.Duration // backoff increment per retry (default 5s)
}

// tunnelManager implements port.TunnelManager.
// Manages the lifecycle of multiple port forwards, with optional auto-reconnect.
type tunnelManager struct {
	mu           sync.Mutex
	forwards     map[string]port.PortForward
	sshClient    *ssh.Client
	reconnect    bool
	reconnectCfg ReconnectConfig
	reconnectFn  func() (*ssh.Client, error) // callback to reconnect (provided by service layer)
	ctx          context.Context
	cancel       context.CancelFunc
	done         chan struct{}
}

// 编译期接口合规检查
var _ port.TunnelManager = (*tunnelManager)(nil)

// NewTunnelManager creates a new tunnel manager.
func NewTunnelManager() *tunnelManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &tunnelManager{
		forwards: make(map[string]port.PortForward),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Add adds a port forward to the manager. Does NOT auto-start.
func (m *tunnelManager) Add(forward port.PortForward) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := forward.ID()
	if _, exists := m.forwards[id]; exists {
		return fmt.Errorf("tunnel forward %s already exists", id)
	}

	m.forwards[id] = forward
	log.Debugf("tunnel forward [%s] added (type=%s)", id, forward.Type())
	return nil
}

// Remove stops and removes a port forward by ID.
func (m *tunnelManager) Remove(id string) error {
	m.mu.Lock()
	f, exists := m.forwards[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("tunnel forward %s not found", id)
	}
	delete(m.forwards, id)
	m.mu.Unlock()

	if err := f.Stop(); err != nil {
		log.Warnf("tunnel forward [%s] stop error: %v", id, err)
		return fmt.Errorf("stop %s: %w", id, err)
	}

	log.Debugf("tunnel forward [%s] removed", id)
	return nil
}

// Get returns a port forward by ID.
func (m *tunnelManager) Get(id string) (port.PortForward, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	f, exists := m.forwards[id]
	if !exists {
		return nil, fmt.Errorf("tunnel forward %s not found", id)
	}
	return f, nil
}

// List returns all registered port forwards.
func (m *tunnelManager) List() []port.PortForward {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]port.PortForward, 0, len(m.forwards))
	for _, f := range m.forwards {
		result = append(result, f)
	}
	return result
}

// StopAll stops all port forwards, clears the map, and closes the SSH client.
func (m *tunnelManager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	for id, f := range m.forwards {
		if err := f.Stop(); err != nil {
			log.Warnf("tunnel forward [%s] stop error: %v", id, err)
			errs = append(errs, fmt.Errorf("stop %s: %w", id, err))
		}
	}

	m.forwards = make(map[string]port.PortForward)

	if m.sshClient != nil {
		if err := m.sshClient.Close(); err != nil {
			log.Warnf("close SSH client: %v", err)
			errs = append(errs, fmt.Errorf("close ssh client: %w", err))
		}
		m.sshClient = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("stop all: %v", errs)
	}
	return nil
}

// StartAll starts all registered forwards using the given SSH client.
// Continues on partial failure and returns a combined error.
func (m *tunnelManager) StartAll(client *ssh.Client) error {
	m.mu.Lock()
	forwards := make([]port.PortForward, 0, len(m.forwards))
	for _, f := range m.forwards {
		forwards = append(forwards, f)
	}
	m.mu.Unlock()

	var errs []error
	for _, f := range forwards {
		if err := f.Start(client); err != nil {
			log.Warnf("tunnel forward [%s] start error: %v", f.ID(), err)
			errs = append(errs, fmt.Errorf("start %s: %w", f.ID(), err))
		} else {
			log.Debugf("tunnel forward [%s] started", f.ID())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("start all: %v", errs)
	}
	return nil
}

// SetReconnect enables auto-reconnection with the given config and reconnect function.
// Starts a background goroutine that monitors the SSH client for disconnection.
// reconnectFn is a callback provided by the service layer that reconnects to the server.
func (m *tunnelManager) SetReconnect(cfg ReconnectConfig, reconnectFn func() (*ssh.Client, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.reconnect {
		// Already running, just update config and callback
		m.reconnectCfg = cfg
		m.reconnectFn = reconnectFn
		return
	}

	m.reconnect = true
	m.reconnectCfg = cfg
	m.reconnectFn = reconnectFn
	m.done = make(chan struct{})

	if cfg.Interval <= 0 {
		m.reconnectCfg.Interval = 5 * time.Second
	}
	if cfg.Backoff <= 0 {
		m.reconnectCfg.Backoff = 5 * time.Second
	}

	go m.reconnectLoop()
}

// SetClient replaces the current SSH client.
func (m *tunnelManager) SetClient(client *ssh.Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sshClient = client
}

// Client returns the current SSH client.
func (m *tunnelManager) Client() *ssh.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sshClient
}

// Close cancels the context and waits for the reconnect loop to finish.
// If no reconnect loop is running, returns immediately.
func (m *tunnelManager) Close() {
	m.cancel()
	if m.done == nil {
		return
	}
	select {
	case <-m.done:
		// normal close
	case <-time.After(5 * time.Second):
		log.Warnf("tunnel manager close timeout")
	}
}

// reconnectLoop monitors the SSH client for disconnection and attempts to reconnect.
// Runs until context is cancelled or max retries exhausted.
func (m *tunnelManager) reconnectLoop() {
	defer close(m.done)

	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		// Wait for a client to be available
		client := m.Client()
		if client == nil {
			select {
			case <-m.ctx.Done():
				return
			case <-time.After(time.Second):
				continue
			}
		}

		// Block until client disconnects or context cancelled
		clientDone := make(chan error, 1)
		go func() {
			clientDone <- client.Wait()
		}()

		select {
		case <-m.ctx.Done():
			return
		case <-clientDone:
			// Client disconnected, try to reconnect
		}

		if !m.tryReconnect() {
			return
		}
	}
}

// tryReconnect attempts to reconnect with exponential backoff.
// Returns true if reconnection succeeded, false if should stop.
func (m *tunnelManager) tryReconnect() bool {
	m.mu.Lock()
	interval := m.reconnectCfg.Interval
	maxRetries := m.reconnectCfg.MaxRetries
	backoff := m.reconnectCfg.Backoff
	reconnectFn := m.reconnectFn
	m.mu.Unlock()

	if reconnectFn == nil {
		log.Warnf("reconnect function not set, giving up")
		return false
	}

	for attempt := 1; maxRetries == 0 || attempt <= maxRetries; attempt++ {
		select {
		case <-m.ctx.Done():
			return false
		case <-time.After(interval):
		}

		retriesStr := "∞"
		if maxRetries > 0 {
			retriesStr = strconv.Itoa(maxRetries)
		}
		log.Infof("reconnecting (attempt %d/%s, interval %v)...", attempt, retriesStr, interval)

		newClient, err := reconnectFn()
		if err != nil {
			log.Warnf("reconnect attempt %d failed: %v", attempt, err)
			interval += backoff
			continue
		}

		// Success - update client and restart forwards
		m.SetClient(newClient)
		if err := m.StartAll(newClient); err != nil {
			log.Warnf("restart forwards after reconnect: %v", err)
		}
		log.Infof("reconnected successfully (attempt %d)", attempt)
		return true
	}

	log.Errorf("max reconnect attempts (%d) exhausted, giving up", maxRetries)
	return false
}

// Daemonize forks the current process to run in background.
// Parent exits with status 0; child continues and returns nil.
//
// In the child process:
//   - Creates a new session (Setsid) to detach from terminal
//   - Redirects stdin/stdout/stderr to /dev/null
//   - Writes current PID to pidPath (if non-empty)
//
// The caller is responsible for blocking the child process after this returns
// (e.g., via WaitForSignal) to keep the daemon alive.
func Daemonize(pidPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}

	// Open /dev/null for redirecting child's stdin/stdout/stderr
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open /dev/null: %w", err)
	}
	defer devNull.Close()

	proc, err := os.StartProcess(exe, os.Args, &os.ProcAttr{
		Files: []*os.File{devNull, devNull, devNull}, // redirect stdin/stdout/stderr to /dev/null
		Sys:   &syscall.SysProcAttr{Setsid: true},    // create new session, detach from terminal
	})
	if err != nil {
		return fmt.Errorf("daemonize: %w", err)
	}

	// Parent process: log child PID and exit
	log.Debugf("daemonized child PID: %d", proc.Pid)
	fmt.Fprintf(os.Stderr, "daemon started (PID: %d)\n", proc.Pid)
	os.Exit(0)

	return nil // unreachable (parent exits above)
}

// WaitForSignal blocks until SIGINT or SIGTERM is received, then executes stop and cleanup.
// stop is the main shutdown function (e.g., stopping all forwards).
// cleanup are optional functions executed after stop (e.g., removing PID file).
func WaitForSignal(stop func() error, cleanup ...func()) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Infof("received signal %v, shutting down...", sig)

	if err := stop(); err != nil {
		log.Errorf("shutdown error: %v", err)
	}

	for _, fn := range cleanup {
		fn()
	}
}

// WritePidFile writes the current process PID to the specified path.
// Ensures the parent directory exists.
func WritePidFile(path string) error {
	expanded, err := filepath.Abs(path)
	if err != nil {
		expanded = path
	}

	dir := filepath.Dir(expanded)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create pid dir %s: %w", dir, err)
	}

	if err := os.WriteFile(expanded, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		return fmt.Errorf("write pid file %s: %w", expanded, err)
	}
	return nil
}

// RemovePidFile removes the PID file if it exists.
func RemovePidFile(path string) {
	expanded, err := filepath.Abs(path)
	if err != nil {
		expanded = path
	}

	if err := os.Remove(expanded); err != nil && !os.IsNotExist(err) {
		log.Warnf("remove pid file %s: %v", expanded, err)
	}
}

// ReadPidFile reads PID from file and returns the corresponding process.
func ReadPidFile(path string) (*os.Process, error) {
	expanded, err := filepath.Abs(path)
	if err != nil {
		expanded = path
	}

	data, err := os.ReadFile(expanded)
	if err != nil {
		return nil, fmt.Errorf("read pid file %s: %w", expanded, err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("parse pid from %s: %w", expanded, err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil, fmt.Errorf("find process %d: %w", pid, err)
	}
	return proc, nil
}
