package proxy

import (
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestSmartProxy_InvalidConfig(t *testing.T) {
	sp := NewSmartProxy(SmartProxyConfig{})
	err := sp.Start(&ssh.Client{})
	if err == nil {
		t.Fatal("expected error for empty config (no listeners)")
	}
}

func TestSmartProxy_NilClient(t *testing.T) {
	sp := NewSmartProxy(SmartProxyConfig{
		SOCKS5Addr: "127.0.0.1:0",
	})
	err := sp.Start(nil)
	if err == nil {
		t.Fatal("expected error for nil ssh client")
	}
}

func TestSmartProxy_AddrReturnsSomething(t *testing.T) {
	sp := NewSmartProxy(SmartProxyConfig{
		SOCKS5Addr: "127.0.0.1:0",
	})
	err := sp.Start(&ssh.Client{})
	if err != nil {
		t.Fatal(err)
	}
	defer sp.Stop()

	addr := sp.Addr()
	if addr == "" {
		t.Error("expected non-empty address with SOCKS5 configured")
	}
}

func TestSmartProxy_AddrFallbackToHTTP(t *testing.T) {
	sp := NewSmartProxy(SmartProxyConfig{
		HTTPAddr: "127.0.0.1:0",
	})
	err := sp.Start(&ssh.Client{})
	if err != nil {
		t.Fatal(err)
	}
	defer sp.Stop()

	addr := sp.Addr()
	if addr == "" {
		t.Error("expected non-empty address from HTTP listener fallback")
	}
}

func TestSmartProxy_StopWithoutStart(t *testing.T) {
	sp := NewSmartProxy(SmartProxyConfig{
		SOCKS5Addr: "127.0.0.1:0",
	})
	err := sp.Stop()
	if err != nil {
		t.Errorf("Stop without Start should not error: %v", err)
	}
}

func TestSmartProxy_DoubleStop(t *testing.T) {
	sp := NewSmartProxy(SmartProxyConfig{
		SOCKS5Addr: "127.0.0.1:0",
	})
	if err := sp.Start(&ssh.Client{}); err != nil {
		t.Fatal(err)
	}
	if err := sp.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := sp.Stop(); err != nil {
		t.Errorf("second Stop should not error: %v", err)
	}
}

func TestSmartProxy_SOCKS5OnlyConfig(t *testing.T) {
	sp := NewSmartProxy(SmartProxyConfig{
		SOCKS5Addr: "127.0.0.1:0",
	})
	if err := sp.Start(&ssh.Client{}); err != nil {
		t.Fatal(err)
	}
	defer sp.Stop()

	addr := sp.Addr()
	if addr == "" {
		t.Error("expected non-empty address")
	}
}

func TestSmartProxy_HTTPOnlyConfig(t *testing.T) {
	sp := NewSmartProxy(SmartProxyConfig{
		HTTPAddr: "127.0.0.1:0",
	})
	if err := sp.Start(&ssh.Client{}); err != nil {
		t.Fatal(err)
	}
	defer sp.Stop()

	addr := sp.Addr()
	if addr == "" {
		t.Error("expected non-empty address")
	}
}

func TestSmartProxy_BothListeners(t *testing.T) {
	sp := NewSmartProxy(SmartProxyConfig{
		SOCKS5Addr: "127.0.0.1:0",
		HTTPAddr:   "127.0.0.1:0",
	})
	if err := sp.Start(&ssh.Client{}); err != nil {
		t.Fatal(err)
	}
	defer sp.Stop()

	addr := sp.Addr()
	if addr == "" {
		t.Error("expected non-empty address")
	}
}

func TestSmartProxy_InvalidBindAddr(t *testing.T) {
	sp := NewSmartProxy(SmartProxyConfig{
		SOCKS5Addr: "999.999.999.999:0",
	})
	err := sp.Start(&ssh.Client{})
	if err == nil {
		t.Error("expected error for invalid bind address")
	}
}
