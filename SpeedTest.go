package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const GetDataIntervalTest = 250 * time.Millisecond

// HTTPTester

type TestOutput struct {
	FastestSP     float64
	BestChunkSize int64
	LowAvgSpeed   float64
	StandardSP    float64
}
type TestConfig struct {
	// 网卡Ping配置
	TCPPingHost     string
	TCPPingPort     int
	TCPPingTimeout  time.Duration
	TCPPingAttempts int

	// SpeedTest配置
	SpeedTestURL      string
	SpeedTestDuration time.Duration
	WarmTestDuration  time.Duration

	// 并发配置
	SingleCurrency int
	MulCurrency    int
}

type NetCardInfoPara struct {
	Name       string
	IP         string
	MultiSpeed float64
	TCPPingMs  int64
}
type NetHTTPTest struct {
	mu          sync.RWMutex
	config      TestConfig
	NetCardInfo map[string]*NetCardInfoPara
}

var NetworkTester = &NetHTTPTest{
	NetCardInfo: make(map[string]*NetCardInfoPara),
}

// DefaultTestConfig 初始化默认配置
func (p *NetHTTPTest) DefaultTestConfig() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = TestConfig{
		TCPPingHost:       "wirelesscdn-download.xuexi.cn",
		TCPPingPort:       443,
		TCPPingTimeout:    5 * time.Second,
		TCPPingAttempts:   5,
		SpeedTestURL:      "https://wirelesscdn-download.xuexi.cn/publish/xuexi_android/latest/xuexi_android_10002068.apk",
		SpeedTestDuration: 3 * time.Second,
		WarmTestDuration:  3 * time.Second,
		SingleCurrency:    1,
		MulCurrency:       10,
	}
	return
}
func (p *NetHTTPTest) InterfaceSearch(IP string) *NetCardInfoPara {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.NetCardInfo[IP]
}
func (p *NetHTTPTest) TCPSpeedTest() {
	//  读取相关信息进行处理，而后写入数据，避免占用过久
	p.mu.Lock()
	defer p.mu.Unlock()
	BestChunkSizeRecorder.mu.Lock()
	defer BestChunkSizeRecorder.mu.Unlock()
	//  遍历字典内容
	Infos := p.NetCardInfo

	for _, InfoAd := range Infos { // 字典获取的不是对应值而是副本
		InfoAd.TCPPingMs = p.TCPLink(InfoAd.IP, p.config.TCPPingHost)
		fmt.Printf("对应IP<%s>To<%s>延迟: %d\n", InfoAd.IP, p.config.TCPPingHost, InfoAd.TCPPingMs)
		bag := p.SpeedTest(InfoAd.IP)
		InfoAd.MultiSpeed = bag.StandardSP
		BestChunkSizeRecorder.content[InfoAd.IP] = bag.BestChunkSize
		NetCardInfo.mu.Lock()
		NetCardInfo.Content[InfoAd.IP].LowAvgSpeed = bag.LowAvgSpeed
		NetCardInfo.Content[InfoAd.IP].FastestSpeed = bag.FastestSP
		NetCardInfo.Content[InfoAd.IP].StandardSpeed = bag.StandardSP
		NetCardInfo.mu.Unlock()
		fmt.Printf("对应IP<%s>To<%s>获取速率: %v\n", InfoAd.IP, p.config.SpeedTestURL, InfoAd.MultiSpeed)
	}
	return
}
func (p *NetHTTPInfo) InitBestProbeClient() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	Info := p.Content
	var BestProbeIP string
	var BestCardSp float64 = 0
	for IP, InfoAd := range Info {
		if BestCardSp < InfoAd.StandardSpeed {
			BestProbeIP = IP
			BestCardSp = InfoAd.StandardSpeed
			fmt.Printf("BestProbeIP: %v, BestCardSp: %v\n", BestProbeIP, BestCardSp)
		}
	}
	return BestProbeIP
}

// TCPLink 测试延迟
func (p *NetHTTPTest) TCPLink(ip string, host string) int64 {
	dialer := &net.Dialer{
		LocalAddr: &net.TCPAddr{IP: net.ParseIP(ip)},
		Timeout:   p.config.TCPPingTimeout,
	}
	var RecordTimeSum int64 = 0
	for _ = range p.config.TCPPingAttempts {
		startTime := time.Now()
		conn, err := dialer.Dial("tcp", fmt.Sprintf("%s:%d", host, p.config.TCPPingPort))
		elapsedTime := time.Since(startTime)
		if err != nil {
			fmt.Printf("出现错误: %v\n", err)
			return -1
		}
		conn.Close()
		DurationTime := elapsedTime.Milliseconds()
		RecordTimeSum += DurationTime
	}
	RecordTimeAvg := RecordTimeSum / int64(p.config.TCPPingAttempts)
	return RecordTimeAvg
}
func (p *NetHTTPTest) SpeedTest(IP string) (outputBag TestOutput) {
	// 获取客户端
	NetCardClient.mu.RLock()
	Client := NetCardClient.Content[IP].ProbeClient
	NetCardClient.mu.RUnlock()

	// 进行有关测试
	// 时间性byte/time记录
	var FastestSpeedAvg float64 = 0
	var byteList []int64
	var totalBytes int64 = 0
	// 重设计算流量速度曲线的计算器
	// GlobalDownloadData.ResetAndStartNewTest(strconv.Itoa(1))

	// 设置暂停器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startTime := time.Now()
	runningTime := time.Now()
	runningTimePit := false

	go func() {
		for {
			time.Sleep(GetDataIntervalTest)
			// 记录byte
			currentTotal := atomic.LoadInt64(&totalBytes)
			// GlobalDownloadData.AddPoint(float64(currentTotal) / 1024 / 1024)
			byteList = append(byteList, currentTotal)
			if runningTimePit {
				// SpeedNow处理
				var SpeedNow float64 = 0
				if len(byteList) > 10 {
					SpeedNow = float64(byteList[len(byteList)-1]-byteList[len(byteList)-4]) / (3 * float64(GetDataIntervalTest.Milliseconds()))
				}

				// SpeedAvg处理
				IntervalFromRun := time.Since(runningTime)
				IntervalFromStart := time.Since(startTime)
				SpeedAvg := float64(currentTotal) / float64(IntervalFromStart.Milliseconds()) // 获取Interval // 计算当前平均时间 b/ms -> /(1024 * 1024) * 1000
				SpeedRunAvg := float64(currentTotal) / float64(IntervalFromRun.Milliseconds())
				if FastestSpeedAvg < SpeedAvg { // 记录最大时间
					FastestSpeedAvg = SpeedAvg
				}
				SpeedAvgStandard := (SpeedRunAvg + SpeedAvg) / 2

				// 判断是否处于一个速度稳定状态
				if SpeedNow != 0 {
					if (SpeedAvgStandard+SpeedNow/20 > SpeedNow && SpeedAvgStandard <= SpeedNow) || (SpeedNow+SpeedAvg/10 > SpeedAvg && SpeedNow <= SpeedAvg) {
						outputBag.LowAvgSpeed = SpeedAvg / (1024 * 1024 / 1000)
						outputBag.BestChunkSize = currentTotal
						outputBag.FastestSP = FastestSpeedAvg / (1024 * 1024 / 1000)
						outputBag.StandardSP = SpeedAvgStandard / (1024 * 1024 / 1000)
						fmt.Printf("SpeedAvg: %v MB/s; SpeedRunAvg: %v\n MB/s SpeedFastestAvg: %v MB/s; BestChunk?: %v MB; SpendTime: %v\n",
							float64(SpeedAvg)/(1024*1024/1000), float64(SpeedRunAvg)/(1024*1024/1000), float64(FastestSpeedAvg)/(1024*1024/1000), float64(currentTotal)/(1024*1024), time.Since(startTime))
						cancel()
						return
					} else {
						fmt.Printf("SpeedAvg: %v MB/s; SpeedAvgRun: %v MB/s\n, SpeedNow: %v MB/s, SpeedStandard: %v MB/s\n", SpeedAvg/(1024*1024/1000), SpeedRunAvg/(1024*1024/1000), SpeedNow/(1024*1024/1000), SpeedAvgStandard/(1024*1024/1000))
					}
				}
			}
			if currentTotal >= 10*1024*1024 { // 保证最小chunk要大于10MB
				if !runningTimePit {
					runningTimePit = true
					runningTime = time.Now()
				}
			}
		}
	}()
	var wg sync.WaitGroup
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		go func() {
			req, err := http.NewRequestWithContext(ctx, "GET", p.config.SpeedTestURL, nil)
			if err != nil {
				fmt.Printf("Req Create Error: %v\n", err)
				return
			}
			resp, err := Client.Do(req)
			defer resp.Body.Close()
			if err != nil {
				fmt.Printf("Client Resp Error: %v\n", err)
			}
			buffer := make([]byte, 2*1024*1024)
			for {
				select {
				case <-ctx.Done():
					wg.Done()
					return
				default:
					n, er := resp.Body.Read(buffer)
					if n > 0 {
						atomic.AddInt64(&totalBytes, int64(n))
					}
					if er != nil {
						wg.Done()
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	return
}
