package proxy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	ctx          context.Context
	cancel       context.CancelFunc
	done         chan struct{}
}

// NewTunnelManager creates a new tunnel manager.
func NewTunnelManager() *tunnelManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &tunnelManager{
		forwards: make(map[string]port.PortForward),
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
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

// SetReconnect enables auto-reconnection with the given config.
func (m *tunnelManager) SetReconnect(cfg ReconnectConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.reconnect = true
	m.reconnectCfg = cfg

	if cfg.Interval <= 0 {
		m.reconnectCfg.Interval = 5 * time.Second
	}
	if cfg.Backoff <= 0 {
		m.reconnectCfg.Backoff = 5 * time.Second
	}
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

// reconnectLoop attempts to reconnect using connectFn and restarts all forwards.
func (m *tunnelManager) reconnectLoop(ctx context.Context, connectFn func() (*ssh.Client, error)) {
	defer func() {
		m.cancel()
		close(m.done)
	}()

	retries := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		client, err := connectFn()
		if err == nil {
			m.mu.Lock()
			m.sshClient = client
			m.mu.Unlock()

			if err := m.StartAll(client); err != nil {
				log.Warnf("reconnect: failed to restart some forwards: %v", err)
			}
			log.Infof("reconnect successful, all forwards restarted")
			return
		}

		retries++
		if m.reconnectCfg.MaxRetries > 0 && retries >= m.reconnectCfg.MaxRetries {
			log.Errorf("reconnect max retries reached (%d)", m.reconnectCfg.MaxRetries)
			m.StopAll()
			return
		}

		waitDur := m.reconnectCfg.Interval + m.reconnectCfg.Backoff*time.Duration(retries-1)
		log.Debugf("reconnect attempt %d failed, retrying in %v", retries, waitDur)

		select {
		case <-ctx.Done():
			return
		case <-time.After(waitDur):
		}
	}
}

// Close cancels the context and waits for the reconnect loop to finish.
// Uses a timeout to prevent deadlock when reconnect loop doesn't close done channel.
func (m *tunnelManager) Close() {
	m.cancel()
	select {
	case <-m.done:
		// normal close
	case <-time.After(5 * time.Second):
		log.Warnf("tunnel manager close timeout")
	}
}

// Daemonize forks the current process to run in background.
// Returns nil in the child process; parent exits with status 0.
func Daemonize() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable: %w", err)
	}

	proc, err := os.StartProcess(exe, os.Args, &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		return fmt.Errorf("daemonize: %w", err)
	}

	log.Debugf("daemonized child PID: %d", proc.Pid)
	os.Exit(0)

	return nil // unreachable
}

// WritePidFile writes the current process PID to the specified path.
// Ensures the parent directory exists.
func WritePidFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create pid dir %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		return fmt.Errorf("write pid file %s: %w", path, err)
	}
	return nil
}

// ReadPidFile reads PID from file and returns the corresponding process.
func ReadPidFile(path string) (*os.Process, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pid file %s: %w", path, err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("parse pid from %s: %w", path, err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil, fmt.Errorf("find process %d: %w", pid, err)
	}
	return proc, nil
}
