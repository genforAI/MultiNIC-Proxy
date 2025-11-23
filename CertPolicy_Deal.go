package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// 流量分流策略（用户自定义策略）

type TrafficAction int

const (
	ActionAccelerate  TrafficAction = iota // 启动分块加速 (Target: 我们的调度器)
	ActionPassThrough                      // 单通道转发 (Target: 目标服务器)
	ActionIsolate                          // 强制走特定网卡 (Target: 目标服务器)
)

type PolicyConfig struct {
	ActionAcc []string `json:"ActionAccelerate"`
	ActionPas []string `json:"ActionPassThrough"`
	ActionIso []string `json:"ActionIso"`
}

type HostPolicy struct {
	Action TrafficAction
	// ForcedNicIP net.IP
}
type PolicyManager struct {
	mu       sync.RWMutex // 允许多读但是只能单写入
	policies map[string]HostPolicy
}

var GlobalPolicyManager = &PolicyManager{
	policies: make(map[string]HostPolicy),
}

func (p *PolicyManager) CheckPolicy(host string) HostPolicy {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 1. 精确匹配
	if policy, ok := p.policies[host]; ok {
		return policy
	}

	// 2. 默认策略（假设已加载，例如 "*": ActionPassThrough）
	if policy, ok := p.policies["*"]; ok {
		return policy
	}

	// 如果默认策略都没找到，返回加速策略以避免崩溃
	return HostPolicy{Action: ActionAccelerate}
}

// LoadPolicies 初始加载Json文件
func (p *PolicyManager) LoadPolicies() error {
	filePath := "./HostPolicy.json"
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var config PolicyConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("wrong Explanation: %s\n", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	for _, host := range config.ActionAcc {
		p.policies[host] = HostPolicy{Action: ActionAccelerate}
	}
	for _, host := range config.ActionPas {
		p.policies[host] = HostPolicy{Action: ActionPassThrough}
	}
	// p.policies["download.test.com"] = HostPolicy{Action: ActionAccelerate}
	p.policies["*"] = HostPolicy{Action: ActionAccelerate} // 默认流量直接转发
	println("Load Done Policy...")
	return nil
}

// 证书缓存处理

type TimeCert struct {
	NotAfter time.Time
	Cert     *tls.Certificate
}
type CertCache struct {
	mu sync.RWMutex
	// Key: Host 域名, Value: 伪造的证书对象
	cache map[string]*TimeCert
}

var GlobalCertCache = &CertCache{
	cache: make(map[string]*TimeCert),
}

// GetTls 从缓存中安全地获取证书 (读操作)
func (c *CertCache) GetTls(host string) (*tls.Certificate, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// 如果host存在对象的话
	if certPtr, ok := c.cache[host]; ok {
		// 判断证书是否过期
		if certPtr.NotAfter.After(time.Now()) {
			return certPtr.Cert, nil
		}
		return nil, fmt.Errorf("certificate Expired")
	}
	return nil, fmt.Errorf("certificate Not Found")
}

// SetTls 将新生成的证书安全地存入缓存 (写操作)
func (c *CertCache) SetTls(host string, certPtr *TimeCert) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[host] = certPtr
	return nil
}
