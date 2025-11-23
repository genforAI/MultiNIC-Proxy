package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

const (
	CADir      = "./certs"
	CarterName = "rootCA.crt"
	CakeyName  = "rootCA.key"
	certPath   = CADir + string(os.PathSeparator) + CarterName
	keyPath    = CADir + string(os.PathSeparator) + CakeyName
)

type CaCertKeyPair struct {
	Cert *x509.Certificate
	Key  *rsa.PrivateKey
}

var caCert = &CaCertKeyPair{
	Cert: nil,
	Key:  nil,
}

func checkRotSrtGen() error {
	// 1. 尝试加载已存在的证书和私钥
	if certBytes, err := os.ReadFile(certPath); err == nil {
		if keyBytes, err := os.ReadFile(keyPath); err == nil {
			// 文件存在，尝试加载
			return caCert.loadCA(certBytes, keyBytes)
		}
	}

	// 2. 如果文件不存在，则生成新的 CA
	fmt.Println("[CA] 根证书文件不存在，正在生成新的根证书和私钥...")
	// 首先创建对应路径目录查询
	if err := os.MkdirAll(CADir, 0700); err != nil {
		return fmt.Errorf("创建目录失败")
	}
	return caCert.generateCA(certPath, keyPath)
}
func (p *CaCertKeyPair) loadCA(certPEM []byte, keyPEM []byte) error {
	// 解析证书 PEM 块
	var err error
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return fmt.Errorf("无法解码 PEM 证书")
	}
	p.Cert, err = x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("解析 X509 证书失败: %w", err)
	}

	// 解析私钥 PEM 块
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return fmt.Errorf("无法解码 PEM 私钥")
	}
	p.Key, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("解析 PKCS1 私钥失败: %w", err)
	}

	fmt.Printf("[CA] 根证书已从文件加载: %s\n", p.Cert.Subject.CommonName)
	return nil
}

// generateCA 用于生成并保存代理的根证书CA和私钥
func (p *CaCertKeyPair) generateCA(certPath, keyPath string) error {
	// 1. 生成 RSA 私钥
	var err error
	p.Key, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("生成 CA 私钥失败: %w", err)
	}

	// 2. 创建证书模板
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			Organization: []string{"Multi-NIC Load Balancer CA"},
			CommonName:   "Multi-NIC Proxy Root CA", // 客户端将看到的名称
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),                 // 有效期10年
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign, // 标记为 CA 证书
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true, // 核心：标记为自签名 CA
	}

	// 3. 自签名证书
	var caCertBytes []byte
	caCertBytes, err = x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &p.Key.PublicKey, p.Key)
	if err != nil {
		return fmt.Errorf("自签名 CA 证书失败: %w", err)
	}
	p.Cert, _ = x509.ParseCertificate(caCertBytes)

	// 4.将私钥和根证书CA写入到当前目录下
	keyFile, _ := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	pem.Encode(keyFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(p.Key),
	})
	keyFile.Close()

	// 写入证书 (PEM 编码)
	certFile, _ := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	pem.Encode(certFile, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertBytes,
	})
	certFile.Close()
	fmt.Printf("[CA] 根证书已生成并保存到 %s\n", certPath)
	return nil
}
