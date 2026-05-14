package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"

	"assh/asshc/port"
	"assh/log"
)

// SmartProxyConfig configures the Smart Proxy system.
type SmartProxyConfig struct {
	SOCKS5Addr  string                          // SOCKS5 listen addr, "" = disabled
	HTTPAddr    string                          // HTTP CONNECT listen addr, "" = disabled
	AuthFunc    func(username, password string) bool // nil = no auth
	RuleEngine  port.RuleEngine                 // nil = all traffic goes through SSH
	ProxyLogger port.ProxyLogger                // nil = no logging
}

// SmartProxy is the unified orchestrator for SOCKS5 + HTTP CONNECT + RuleEngine + ProxyLogger.
type SmartProxy struct {
	mu             sync.Mutex
	cfg            SmartProxyConfig
	client         *ssh.Client
	socks5Listener net.Listener
	httpListener   net.Listener
	active         map[net.Conn]struct{}
	sessionCounter int64
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	socks5Helper   *SOCKS5Proxy
	httpHelper     *HTTPConnectProxy
}

// NewSmartProxy creates a SmartProxy with the given config.
func NewSmartProxy(cfg SmartProxyConfig) *SmartProxy {
	ctx, cancel := context.WithCancel(context.Background())

	var socks5Helper *SOCKS5Proxy
	var httpHelper *HTTPConnectProxy
	if cfg.AuthFunc != nil {
		socks5Helper = NewSOCKS5ProxyWithAuth(cfg.AuthFunc)
		httpHelper = NewHTTPConnectProxyWithAuth(cfg.AuthFunc)
	} else {
		socks5Helper = NewSOCKS5Proxy()
		httpHelper = NewHTTPConnectProxy()
	}

	return &SmartProxy{
		cfg:          cfg,
		active:       make(map[net.Conn]struct{}),
		sessionCounter: 0,
		ctx:          ctx,
		cancel:       cancel,
		socks5Helper: socks5Helper,
		httpHelper:   httpHelper,
	}
}

// Start begins listening on configured addresses.
func (sp *SmartProxy) Start(client *ssh.Client) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if client == nil {
		return fmt.Errorf("ssh client is nil")
	}
	sp.client = client

	if sp.cfg.SOCKS5Addr != "" {
		listener, err := net.Listen("tcp", sp.cfg.SOCKS5Addr)
		if err != nil {
			return fmt.Errorf("socks5 listen: %w", err)
		}
		sp.socks5Listener = listener
		sp.wg.Add(1)
		go sp.acceptLoop(listener, "socks5")
		log.Infof("SmartProxy SOCKS5 listening on %s", listener.Addr().String())
	}

	if sp.cfg.HTTPAddr != "" {
		listener, err := net.Listen("tcp", sp.cfg.HTTPAddr)
		if err != nil {
			if sp.socks5Listener != nil {
				sp.socks5Listener.Close()
			}
			return fmt.Errorf("http listen: %w", err)
		}
		sp.httpListener = listener
		sp.wg.Add(1)
		go sp.acceptLoop(listener, "http")
		log.Infof("SmartProxy HTTP listening on %s", listener.Addr().String())
	}

	if sp.socks5Listener == nil && sp.httpListener == nil {
		return fmt.Errorf("no listener configured (set SOCKS5Addr and/or HTTPAddr)")
	}

	return nil
}

// Stop terminates all listeners and active connections.
func (sp *SmartProxy) Stop() error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if sp.socks5Listener != nil {
		sp.socks5Listener.Close()
	}
	if sp.httpListener != nil {
		sp.httpListener.Close()
	}

	for conn := range sp.active {
		conn.Close()
	}
	sp.active = make(map[net.Conn]struct{})

	sp.client = nil
	sp.cancel()

	return nil
}

// Addr returns the address of a specific listener (SOCKS5 preferred, fallback to HTTP).
func (sp *SmartProxy) Addr() string {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	if sp.socks5Listener != nil {
		return sp.socks5Listener.Addr().String()
	}
	if sp.httpListener != nil {
		return sp.httpListener.Addr().String()
	}
	return ""
}

// acceptLoop accepts connections and dispatches them to handleConnection.
func (sp *SmartProxy) acceptLoop(listener net.Listener, protocol string) {
	defer sp.wg.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-sp.ctx.Done():
				return
			default:
				log.Debugf("SmartProxy accept error (%s): %v", protocol, err)
				return
			}
		}

		sp.mu.Lock()
		sp.active[conn] = struct{}{}
		sp.mu.Unlock()

		sp.wg.Add(1)
		go func() {
			defer func() {
				sp.mu.Lock()
				delete(sp.active, conn)
				sp.mu.Unlock()
				sp.wg.Done()
			}()
			sp.handleConnection(conn, protocol)
		}()
	}
}

// handleConnection processes a single proxy connection through the pipeline:
// 1. Protocol handshake 2. Rule Engine routing 3. Dial target 4. Bidirectional copy 5. Logging.
func (sp *SmartProxy) handleConnection(conn net.Conn, protocol string) {
	defer conn.Close()

	startTime := time.Now()
	sessionID := fmt.Sprintf("%s-%d", protocol, atomic.AddInt64(&sp.sessionCounter, 1))

	var targetAddr string
	var handshakeErr error

	switch protocol {
	case "socks5":
		if err := sp.socks5Helper.socksHandshake(conn); err != nil {
			log.Debugf("[%s] SOCKS5 handshake failed: %v", sessionID, err)
			return
		}
		targetAddr, handshakeErr = sp.socks5Helper.readRequest(conn)
	case "http":
		targetAddr, handshakeErr = sp.httpHelper.httpHandshake(conn)
	default:
		return
	}
	if handshakeErr != nil {
		log.Debugf("[%s] handshake failed: %v", sessionID, handshakeErr)
		return
	}

	routeAction := "proxy"
	var matchedRule *port.MatchedRule

	if sp.cfg.RuleEngine != nil {
		useProxy, rule, err := sp.cfg.RuleEngine.Match(targetAddr)
		if err != nil {
			log.Debugf("[%s] rule engine error: %v (defaulting to proxy)", sessionID, err)
		} else {
			if rule != nil {
				matchedRule = rule
				routeAction = rule.Action
			} else if !useProxy {
				routeAction = "direct"
			}
		}
	}

	var remoteConn net.Conn
	var dialErr error

	switch routeAction {
	case "proxy":
		remoteConn, dialErr = sp.client.Dial("tcp", targetAddr)
	case "direct":
		remoteConn, dialErr = net.DialTimeout("tcp", targetAddr, 10*time.Second)
	}
	if dialErr != nil {
		log.Debugf("[%s] dial %s (%s): %v", sessionID, targetAddr, routeAction, dialErr)
		if protocol == "socks5" {
			sp.socks5Helper.sendReply(conn, socksRepUnreachable)
		} else if protocol == "http" {
			sp.httpHelper.writeError(conn, "502", "Bad Gateway")
		}
		return
	}
	defer remoteConn.Close()

	if protocol == "socks5" {
		if err := sp.socks5Helper.sendReply(conn, socksRepSuccess); err != nil {
			log.Debugf("[%s] send SOCKS5 reply: %v", sessionID, err)
			return
		}
	} else if protocol == "http" {
		if err := sp.httpHelper.writeResponse(conn, "200", "Connection Established"); err != nil {
			log.Debugf("[%s] send HTTP response: %v", sessionID, err)
			return
		}
	}

	var sent, recv int64
	done := make(chan struct{})

	go func() {
		n, _ := io.Copy(conn, remoteConn)
		atomic.AddInt64(&recv, n)
		conn.Close()
		close(done)
	}()

	n, _ := io.Copy(remoteConn, conn)
	atomic.AddInt64(&sent, n)
	remoteConn.Close()
	<-done

	if sp.cfg.ProxyLogger != nil {
		duration := time.Since(startTime)
		ruleMatched := ""
		if matchedRule != nil {
			ruleMatched = matchedRule.Pattern
		}

		logEntry := &port.RequestLog{
			SessionID:   sessionID,
			Timestamp:   startTime,
			Protocol:    protocol,
			ClientAddr:  conn.RemoteAddr().String(),
			TargetAddr:  targetAddr,
			Action:      routeAction,
			RuleMatched: ruleMatched,
			BytesSent:   atomic.LoadInt64(&sent),
			BytesRecv:   atomic.LoadInt64(&recv),
			Duration:    duration,
		}

		if err := sp.cfg.ProxyLogger.LogRequest(logEntry); err != nil {
			log.Debugf("[%s] log error: %v", sessionID, err)
		}
	}
}
