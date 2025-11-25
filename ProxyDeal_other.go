//go:build !windows
// +build !windows

package main

import (
	"fmt"
	"runtime"
	"sync"
)

type ProxyManager struct {
	LocalPort int
	Addr      string
	mu        sync.RWMutex
}

var proxyMgr = &ProxyManager{
	Addr:      "127.0.0.1",
	LocalPort: 10808,
}

func (pm *ProxyManager) SetSystemProxy(enable bool) error {
	return fmt.Errorf("system proxy management is not supported on %s", runtime.GOOS)
}

func (pm *ProxyManager) setSystemProxy(enable bool, port int) error {
	return fmt.Errorf("system proxy management is not supported on %s", runtime.GOOS)
}

func (pm *ProxyManager) GetLocalPort() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.LocalPort
}

func (pm *ProxyManager) SetLocalPort(Addr string, port int) error {
	pm.mu.Lock()
	pm.Addr = Addr
	pm.LocalPort = port
	pm.mu.Unlock()
	return nil
}

func StartSystemProxy(Addr string, port int) error {
	proxyMgr.SetLocalPort(Addr, port)
	// On non-Windows systems, we just log that system proxy is not being set
	fmt.Printf("⚠️ System proxy management is not supported on %s. Please configure your proxy manually to %s:%d\n", runtime.GOOS, Addr, port)
	return nil
}

func EndSystemProxy() error {
	// On non-Windows systems, we just log that system proxy is not being unset
	fmt.Printf("⚠️ System proxy management is not supported on %s. Please unconfigure your proxy manually.\n", runtime.GOOS)
	return nil
}
