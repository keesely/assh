package service

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	"assh/asshc/domain"
	"assh/asshc/port"
	"assh/asshc/infra/proxy"
	"assh/log"
)

type ProxyOptions struct {
	SOCKS5Addr    string
	HTTPAddr      string
	Reverse       bool
	ReverseTCP    bool
	ReverseHTTP   bool
	Auth          string
	Ports         string
	RuleFile      string
	LogDir        string
	Daemon        bool
	AutoReconnect string
}

type ForwardSpec struct {
	LocalAddr  string
	RemoteAddr string
	Reverse    bool
}

type tunnelManagerExt interface {
	StartAll(client *ssh.Client) error
	SetClient(client *ssh.Client)
	SetReconnect(cfg proxy.ReconnectConfig)
}

type ProxyService struct {
	repo        port.ServerRepository
	connector   port.SSHConnector
	connectSvc  *ConnectService
	tunnelMgr   port.TunnelManager
	smartProxy  *proxy.SmartProxy
	socks5Proxy *proxy.SOCKS5Proxy
	httpProxy   *proxy.HTTPConnectProxy
	ruleEngine  port.RuleEngine
	proxyLogger port.ProxyLogger
	sshClient   *ssh.Client
	mu          sync.Mutex
}

func NewProxyService(
	repo port.ServerRepository,
	connector port.SSHConnector,
	connectSvc *ConnectService,
	tunnelMgr port.TunnelManager,
) *ProxyService {
	return &ProxyService{
		repo:       repo,
		connector:  connector,
		connectSvc: connectSvc,
		tunnelMgr:  tunnelMgr,
	}
}

func (s *ProxyService) StartProxy(name string, opts ProxyOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sshClient != nil {
		return fmt.Errorf("proxy already running")
	}

	server, err := s.repo.Get(name)
	if err != nil {
		return fmt.Errorf("get server %s: %w", name, err)
	}
	if server == nil {
		return domain.ErrNotFound
	}

	client, err := s.connector.Connect(server)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", name, err)
	}
	s.sshClient = client

	return s.startProxyWithClient(client, opts)
}

func (s *ProxyService) StartDirectProxy(host string, port int, user, password, keyFile string, opts ProxyOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sshClient != nil {
		return fmt.Errorf("proxy already running")
	}

	client, err := s.connectSvc.ConnectDirect(host, port, user, password, keyFile, "")
	if err != nil {
		return fmt.Errorf("connect direct: %w", err)
	}
	s.sshClient = client

	return s.startProxyWithClient(client, opts)
}

func (s *ProxyService) startProxyWithClient(client *ssh.Client, opts ProxyOptions) error {
	if opts.Reverse {
		return s.startReverseMode(client, opts)
	}

	authFn := parseAuth(opts.Auth)

	useSmartProxy := opts.RuleFile != "" || opts.LogDir != ""

	socks5Addr := opts.SOCKS5Addr
	httpAddr := opts.HTTPAddr

	if socks5Addr == "" && httpAddr == "" {
		socks5Addr = ":1080"
	}

	if useSmartProxy {
		if opts.RuleFile != "" {
			re := proxy.NewRuleEngine(true)
			if err := re.Load(opts.RuleFile); err != nil {
				log.Warnf("failed to load rules from %s: %v (all traffic will use proxy)", opts.RuleFile, err)
			}
			s.ruleEngine = re
		}

		if opts.LogDir != "" {
			pl, err := proxy.NewProxyLogger(opts.LogDir)
			if err != nil {
				log.Warnf("failed to create proxy logger: %v (logging disabled)", err)
			} else {
				s.proxyLogger = pl
			}
		}

		cfg := proxy.SmartProxyConfig{
			SOCKS5Addr:  socks5Addr,
			HTTPAddr:    httpAddr,
			AuthFunc:    authFn,
			RuleEngine:  s.ruleEngine,
			ProxyLogger: s.proxyLogger,
		}

		sp := proxy.NewSmartProxy(cfg)
		if err := sp.Start(client); err != nil {
			return fmt.Errorf("start smart proxy: %w", err)
		}
		s.smartProxy = sp
	} else {
		if socks5Addr != "" {
			var p *proxy.SOCKS5Proxy
			if authFn != nil {
				p = proxy.NewSOCKS5ProxyWithAuth(authFn)
			} else {
				p = proxy.NewSOCKS5Proxy()
			}
			if err := p.Start(client, socks5Addr); err != nil {
				return fmt.Errorf("start socks5 proxy: %w", err)
			}
			s.socks5Proxy = p
		}

		if httpAddr != "" {
			var p *proxy.HTTPConnectProxy
			if authFn != nil {
				p = proxy.NewHTTPConnectProxyWithAuth(authFn)
			} else {
				p = proxy.NewHTTPConnectProxy()
			}
			if err := p.Start(client, httpAddr); err != nil {
				if s.socks5Proxy != nil {
					s.socks5Proxy.Stop()
				}
				return fmt.Errorf("start http proxy: %w", err)
			}
			s.httpProxy = p
		}
	}

	if opts.AutoReconnect != "" {
		enabled, cfg := parseAutoReconnect(opts.AutoReconnect)
		if enabled {
			if tm, ok := s.tunnelMgr.(tunnelManagerExt); ok {
				tm.SetReconnect(cfg)
			}
		}
	}

	if opts.Daemon {
		if err := proxy.Daemonize(); err != nil {
			log.Warnf("daemonize failed: %v", err)
		}
	}

	if !opts.Daemon {
		s.mu.Unlock()
		waitForSignal(s.StopProxy)
		s.mu.Lock()
	}

	return nil
}

func (s *ProxyService) startReverseMode(client *ssh.Client, opts ProxyOptions) error {
	if opts.Ports == "" {
		return fmt.Errorf("reverse mode requires --ports")
	}

	rules, err := ParsePorts(opts.Ports)
	if err != nil {
		return fmt.Errorf("parse ports: %w", err)
	}

	authFn := parseAuth(opts.Auth)

	for _, rule := range rules {
		for i := range rule.ServerPorts {
			serverPort := rule.ServerPorts[i]
			localPort := rule.LocalPorts[i]

			remoteAddr := fmt.Sprintf("0.0.0.0:%d", serverPort)
			localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)

			if opts.ReverseTCP && authFn != nil {
				rf := proxy.NewRemoteForward(remoteAddr, localAddr)
				if err := s.tunnelMgr.Add(rf); err != nil {
					log.Warnf("add reverse forward %s->%s: %v", remoteAddr, localAddr, err)
					continue
				}
			} else if opts.ReverseHTTP && authFn != nil {
				rf := proxy.NewRemoteForward(remoteAddr, localAddr)
				if err := s.tunnelMgr.Add(rf); err != nil {
					log.Warnf("add reverse forward %s->%s: %v", remoteAddr, localAddr, err)
					continue
				}
			} else {
				rf := proxy.NewRemoteForward(remoteAddr, localAddr)
				if err := s.tunnelMgr.Add(rf); err != nil {
					log.Warnf("add reverse forward %s->%s: %v", remoteAddr, localAddr, err)
					continue
				}
			}
		}
	}

	if s.tunnelMgr != nil {
		if tm, ok := s.tunnelMgr.(tunnelManagerExt); ok {
			tm.SetClient(client)
			if err := tm.StartAll(client); err != nil {
				return fmt.Errorf("start tunnels: %w", err)
			}
		}
	}

	return nil
}

func (s *ProxyService) StopProxy() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error

	if s.smartProxy != nil {
		if err := s.smartProxy.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("stop smart proxy: %w", err))
		}
		s.smartProxy = nil
	}

	if s.socks5Proxy != nil {
		if err := s.socks5Proxy.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("stop socks5 proxy: %w", err))
		}
		s.socks5Proxy = nil
	}

	if s.httpProxy != nil {
		if err := s.httpProxy.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("stop http proxy: %w", err))
		}
		s.httpProxy = nil
	}

	if s.proxyLogger != nil {
		if err := s.proxyLogger.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close proxy logger: %w", err))
		}
		s.proxyLogger = nil
	}

	s.ruleEngine = nil

	if s.tunnelMgr != nil {
		if err := s.tunnelMgr.StopAll(); err != nil {
			errs = append(errs, fmt.Errorf("stop tunnels: %w", err))
		}
	}

	if s.sshClient != nil {
		if err := s.connector.Close(s.sshClient); err != nil {
			errs = append(errs, fmt.Errorf("close ssh client: %w", err))
		}
		s.sshClient = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("stop proxy: %v", errs)
	}

	log.Infof("proxy stopped")
	return nil
}

func (s *ProxyService) StartTunnel(name string, forwards []ForwardSpec, opts ProxyOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sshClient != nil {
		return fmt.Errorf("tunnel already active")
	}

	server, err := s.repo.Get(name)
	if err != nil {
		return fmt.Errorf("get server %s: %w", name, err)
	}
	if server == nil {
		return domain.ErrNotFound
	}

	client, err := s.connector.Connect(server)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", name, err)
	}
	s.sshClient = client

	for _, spec := range forwards {
		var fw port.PortForward
		if spec.Reverse {
			fw = proxy.NewRemoteForward(spec.RemoteAddr, spec.LocalAddr)
		} else {
			fw = proxy.NewLocalForward(spec.LocalAddr, spec.RemoteAddr)
		}

		if err := s.tunnelMgr.Add(fw); err != nil {
			log.Warnf("add forward %s: %v", fw.ID(), err)
			continue
		}
	}

	if tm, ok := s.tunnelMgr.(tunnelManagerExt); ok {
		tm.SetClient(client)
		if err := tm.StartAll(client); err != nil {
			return fmt.Errorf("start tunnels: %w", err)
		}

		if opts.AutoReconnect != "" {
			enabled, cfg := parseAutoReconnect(opts.AutoReconnect)
			if enabled {
				tm.SetReconnect(cfg)
			}
		}
	}

	if opts.Daemon {
		proxy.Daemonize()
	}

	if !opts.Daemon {
		s.mu.Unlock()
		waitForSignal(func() error {
			return s.tunnelMgr.StopAll()
		})
		s.mu.Lock()
	}

	return nil
}

func (s *ProxyService) StopTunnel(id string) error {
	return s.tunnelMgr.Remove(id)
}

func (s *ProxyService) ListTunnels() []port.PortForward {
	return s.tunnelMgr.List()
}

func (s *ProxyService) ReloadRules() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ruleEngine == nil {
		return fmt.Errorf("no rule engine active")
	}
	return s.ruleEngine.Reload()
}

func parseAuth(authStr string) func(username, password string) bool {
	if authStr == "" {
		return nil
	}
	parts := strings.SplitN(authStr, ":", 2)
	if len(parts) != 2 {
		return nil
	}
	user, pass := parts[0], parts[1]
	return func(username, password string) bool {
		return username == user && password == pass
	}
}

func parseAutoReconnect(s string) (bool, proxy.ReconnectConfig) {
	if s == "" {
		return false, proxy.ReconnectConfig{}
	}

	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return false, proxy.ReconnectConfig{}
	}

	retriesStr, intervalStr := parts[0], parts[1]

	maxRetries := 0
	if retriesStr != "" {
		if r, err := strconv.Atoi(retriesStr); err == nil {
			maxRetries = r
		}
	}

	var interval time.Duration
	if intervalStr != "" {
		if d, err := time.ParseDuration(intervalStr); err == nil {
			interval = d
		}
	}

	if interval <= 0 {
		interval = 5 * time.Second
	}

	return true, proxy.ReconnectConfig{
		MaxRetries: maxRetries,
		Interval:   interval,
		Backoff:    5 * time.Second,
	}
}

func waitForSignal(stop func() error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Infof("received signal %v, shutting down...", sig)
	if err := stop(); err != nil {
		log.Errorf("shutdown error: %v", err)
	}
}
