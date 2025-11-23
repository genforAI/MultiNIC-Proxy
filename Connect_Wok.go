package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

func Listener(wg *sync.WaitGroup, addr string, MonitorCtx context.Context) {
	defer wg.Done()
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listening on %s: %s\n", addr, err)
		return
	}
	fmt.Printf("Listening on %s\n", addr)
	for {
		select {
		case <-MonitorCtx.Done():
			return
		default:
		}
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		go handleConnection(conn)
	}
}
func handleConnection(conn net.Conn) {
	// 读取Connection
	reader := bufio.NewReader(conn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		// 一般来说，这里的req是不可能能在开始的时候接收到io.EOF的？除非出现异常错误
		fmt.Printf("客户端请求出现情况: %v\n", err)
		conn.Close()
		return
	}
	// 解析Host获取对应策略以及对应端口
	Host := req.Host
	if Host == "" {
		if req.URL.Host == "" {
			fmt.Println("Host is empty")
			conn.Close()
			return
		} else {
			Host = req.URL.Host
		}
	}
	policy := GlobalPolicyManager.CheckPolicy(Host)
	// 进行相关性的连接
	if req.Method == "CONNECT" {
		_, err := io.WriteString(conn, "HTTP/1.1 200 Connection established\r\n\r\n")
		if err != nil {
			fmt.Printf("客户端连接响应失败: %v\n", err)
			conn.Close()
			return
		}
		if policy.Action == ActionAccelerate {
			//go WarmProbeClient(req.Host)
			tlsConn, err := tlsShake(conn)
			if err != nil {
				conn.Close()
				return
			}
			// fmt.Println("完成握手并即将开始进行请求处理")
			go HttpsHandle(tlsConn)
		} else {
			HostPort := net.JoinHostPort(Host, "443")
			targetConn, err := net.Dial("tcp", HostPort)
			if err != nil {
				fmt.Printf("连接服务器出现错误: %v\n", err)
				conn.Close()
				return
			}
			WithoutTlsStraight(conn, targetConn)
		}
	} else {
		HostPort := net.JoinHostPort(Host, "80")
		targetConn, err := net.Dial("tcp", HostPort)
		if err != nil {
			fmt.Printf("连接服务器出现错误: %v\n", err)
			conn.Close()
			return
		}
		WithoutTlsStraight(conn, targetConn)
	}
	return
}
func tlsShake(conn net.Conn) (net.Conn, error) {
	tlsConfig := &tls.Config{
		GetCertificate: certHandler,
		NextProtos:     []string{"http/1.1"},
	}
	// 将原本连接封装成TLS服务器
	tlsServerConn := tls.Server(conn, tlsConfig)

	// 进行tls握手测试
	if err := tlsServerConn.Handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("客户端握手失败: %w", err)
	}
	//fmt.Printf("客户端握手成功")

	return tlsServerConn, nil
}

func certHandler(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	SNI := clientHello.ServerName
	if SNI == "" {
		// 如果客户端未提供 SNI (很少见)，则使用一个默认或直接拒绝
		return nil, fmt.Errorf("客户端未提供 SNI 域名")
	}
	TargetSNI := net.JoinHostPort(SNI, "443")

	// 2. 检查缓存
	if cachedCert, err := GlobalCertCache.GetTls(TargetSNI); err == nil {
		// fmt.Printf("[CERT] 命中缓存: %s\n", hostport)
		// 检查到期时间
		return cachedCert, nil
	}

	// 3. 针对伪造证书的创建
	targetConn, err := net.Dial("tcp", TargetSNI)
	if err != nil {
		return nil, fmt.Errorf("无法连接到目标服务器 %s: %w", TargetSNI, err)
	}
	// 创建简单tls连接并返回证书
	host, _, _ := net.SplitHostPort(TargetSNI)
	targetTLSConn := tls.Client(targetConn, &tls.Config{InsecureSkipVerify: true, ServerName: host})
	if err := targetTLSConn.Handshake(); err != nil {
		return nil, fmt.Errorf("和服务器握手失败: %w", err)
	}
	// 开始提取真实服务器证书
	targetState := targetTLSConn.ConnectionState()
	if len(targetState.PeerCertificates) == 0 {
		return nil, fmt.Errorf("未能够从目标服务器获取证书")
	}
	targetCert := targetState.PeerCertificates[0]
	fmt.Println(targetState.NegotiatedProtocol)
	// printTargetCertificate(targetCert)
	defer targetConn.Close()
	defer targetTLSConn.Close()

	// 检测对应网站证书：
	// 开始伪造证书并进行签名
	// 创建伪造证书的模板
	certTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   targetCert.Subject.CommonName, // 使用真实的 CommonName
			Organization: targetCert.Subject.Organization,
		},
		DNSNames: targetCert.DNSNames, // 复制所有 SANs/DNSNames，确保兼容性

		// 关键：有效期使用真实证书的有效期（或更短）
		NotBefore: targetCert.NotBefore,
		NotAfter:  targetCert.NotAfter,

		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,

		// Extensions: []pkix.Extension{},
	}

	// 创建证书以及对应签名
	fakeCertBytes, err := x509.CreateCertificate(rand.Reader, certTmpl, caCert.Cert, &caCert.Key.PublicKey, caCert.Key)
	if err != nil {
		return nil, fmt.Errorf("伪造证书签名失败: %w", err)
	}
	// 创建tls证书对象
	parsedFakeCert, err := x509.ParseCertificate(fakeCertBytes)
	fakeCert := &tls.Certificate{
		Certificate: [][]byte{fakeCertBytes, caCert.Cert.Raw},
		PrivateKey:  caCert.Key,
		Leaf:        parsedFakeCert,
	}
	// 证书写入缓存，后面调用使用地址调用
	certPtr := &TimeCert{
		NotAfter: targetCert.NotAfter,
		Cert:     fakeCert,
	}
	GlobalCertCache.SetTls(TargetSNI, certPtr)
	return fakeCert, nil
}
func WithoutTlsStraight(conn net.Conn, targetConn net.Conn) {
	var wg sync.WaitGroup
	defer conn.Close()
	defer targetConn.Close()
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(targetConn, conn); err != nil && err != io.EOF {
			// 客户端 -> 服务器
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := io.Copy(conn, targetConn); err != nil && err != io.EOF {
			// server -> client
		}
	}()
	wg.Wait()
}

//func WarmProbeClient(host string) {
//	// 设置对应URL
//	url := "https://" + host + "/"
//	NetCardClient.mu.RLock()
//	defer NetCardClient.mu.RUnlock()
//	NetCardClientMap := NetCardClient.Content
//	for _, ClientsAd := range NetCardClientMap {
//		Client := ClientsAd.ProbeClient
//		req, err := http.NewRequest("GET", url, nil)
//		if err != nil {
//			fmt.Printf("预热-创建GET请求失败: %v\n", err)
//			return
//		}
//		resp, err := Client.Do(req)
//		if err != nil {
//			fmt.Printf("预热-请求失败: %v\n", err)
//			return
//		}
//		io.Copy(io.Discard, resp.Body)
//		resp.Body.Close()
//	}
//	return
//}
