//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
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
	port := pm.GetLocalPort()
	if port <= 0 {
		return fmt.Errorf("invalid port: %d", port)
	}
	return pm.setSystemProxy(enable, port)
}
func (pm *ProxyManager) setSystemProxy(enable bool, port int) error {
	proxyServer := fmt.Sprintf("127.0.0.1:%d", port)
	if enable {
		// 启用代理
		cmd := exec.Command("reg", "add",
			"HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings",
			"/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "1", "/f")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to enable proxy: %w", err)
		}

		cmd = exec.Command("reg", "add",
			"HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings",
			"/v", "ProxyServer", "/d", proxyServer, "/f")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set proxy server: %w", err)
		}

		fmt.Printf("✓ Windows proxy enabled: %s\n", proxyServer)
	} else {
		// 禁用代理
		cmd := exec.Command("reg", "add",
			"HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Internet Settings",
			"/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "0", "/f")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to disable proxy: %w", err)
		}

		fmt.Println("✓ Windows proxy disabled")
	}
	return nil
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
	if err := proxyMgr.SetSystemProxy(true); err != nil {
		return fmt.Errorf("start system proxy failed: %v", err)
	}
	return nil
}
func EndSystemProxy() error {
	if err := proxyMgr.SetSystemProxy(false); err != nil {
		return fmt.Errorf("end system proxy failed: %v", err)
	}
	return nil
}
