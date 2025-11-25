package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
)

const CheckInterval = 500 * time.Millisecond
const ClientComNum = 2

// 纯进行记录的原子bytes

type ClientBytes struct {
	clientChunks0Bytes atomic.Int64
	clientChunks1Bytes atomic.Int64
	clientProbeBytes   atomic.Int64
}
type ClientBytesRecorder struct {
	content sync.Map
}

// 提供对应最优IP/Probe/Chunks获取

type NetCardHTTPCho struct {
	ChunksEntries []ChunksClientEntry
	TotalChunks   float64
	ProbeEntries  []ProbeClientEntry
	TotalProbe    float64
}
type ChunksClientEntry struct {
	IP      string
	Index   int
	ProbNum float64
}
type ProbeClientEntry struct {
	IP      string
	ProbNum float64
}

// HTTPComInfo记录对应信息进行有关的计算

type NetCardClientInfo struct {
	//modelSpeed    float64
	bytesInterval int64
}
type NetCardHTTPInfoAdd struct {
	EachClient    []*NetCardClientInfo // 其中第一个是Probe，随后的是Chunks
	LowAvgSpeed   float64
	Time          time.Time
	StandardSpeed float64
	FastestSpeed  float64
}

// HTTPClient保存对应创建client地址

type NetCardHTTPClient struct {
	CommonClient []*http.Client
	ProbeClient  *http.Client
}

// 基础结构

// BestChunkSizeFrame 用于记录对应网卡的最优chunkSize
type BestChunkSizeFrame struct {
	content map[string]int64
	mu      sync.RWMutex
}
type NetHTTPCho struct {
	current atomic.Pointer[NetCardHTTPCho]
}
type NetHTTPInfo struct {
	mu      sync.RWMutex
	Content map[string]*NetCardHTTPInfoAdd
}

type NetHTTPClient struct {
	mu      sync.RWMutex
	Content map[string]*NetCardHTTPClient
}

// 下面部分实例

var NetCardBytes = &ClientBytesRecorder{}
var BestChunkSizeRecorder = &BestChunkSizeFrame{
	content: make(map[string]int64),
}
var NetCardCho = &NetHTTPCho{}
var NetCardInfo = &NetHTTPInfo{
	Content: make(map[string]*NetCardHTTPInfoAdd),
}
var NetCardClient = &NetHTTPClient{
	Content: make(map[string]*NetCardHTTPClient),
}

func (r *ClientBytesRecorder) GetOrCreate(ip string) *ClientBytes {
	actual, _ := r.content.LoadOrStore(ip, &ClientBytes{})
	return actual.(*ClientBytes)
}

// InitNetCardInfo 初始化检测网卡
func InitNetCardInfo() {
	NetworkTester.mu.Lock()
	defer NetworkTester.mu.Unlock()
	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Printf("找不到网卡信息: %v\n", err)
		return
	}
	for _, ifaces := range interfaces {
		if ifaces.Flags&net.FlagUp == 0 {
			continue
		}
		NetInfoTester := &NetCardInfoPara{}
		addrs, er := ifaces.Addrs()
		if er == nil {
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
					IP := ipnet.IP.String()
					NetInfoTester.IP = IP
				}
			}
		}
		if len(NetInfoTester.IP) > 0 {
			NetworkTester.NetCardInfo[NetInfoTester.IP] = NetInfoTester
		}
	}
	return
}

// IPHTTPClientAndChoInit 绑定对应HTTP客户端 && 同时进行初始化ClientEntry保证开始阶段可以正常进行读取
func IPHTTPClientAndChoInit() {
	// 只有一开始写入client的时候才会使用的Mu
	var EntriesChunks []ChunksClientEntry
	var EntriesProbe []ProbeClientEntry
	NetworkTester.mu.RLock()
	defer NetworkTester.mu.RUnlock()
	Content := NetworkTester.NetCardInfo
	for LocalIP := range Content {
		fmt.Printf("发现当前IP: %s\n", LocalIP)
		TransportPoolCreate(LocalIP)
		EntriesChunks = append(EntriesChunks,
			ChunksClientEntry{
				IP:    LocalIP,
				Index: 0,
			})
		EntriesProbe = append(EntriesProbe,
			ProbeClientEntry{
				IP: LocalIP,
			})
	}
	// 初始化传递性切片
	NetCardCho.current.Store(
		&NetCardHTTPCho{
			ChunksEntries: EntriesChunks,
			TotalChunks:   0,
			ProbeEntries:  EntriesProbe,
			TotalProbe:    0,
		})
	return
}
func TransportPoolCreate(LocalIP string) {
	var ClientInfo []*NetCardClientInfo
	// 检测文件大小所需连接特征：连接短，不需要过多网速要求，不采用特殊动态调配
	probeTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second, // Host连接超时时间
			KeepAlive: 30 * time.Second, // 发送连接包维持连接间隔
			LocalAddr: &net.TCPAddr{
				IP: net.ParseIP(LocalIP),
			},
		}).DialContext,
		TLSClientConfig: &tls.Config{
			NextProtos:         []string{"h2", "http/1.1"},
			ClientSessionCache: tls.NewLRUClientSessionCache(128),
		},
		ForceAttemptHTTP2:   true,
		MaxIdleConnsPerHost: 10, // 每个Host的最大连接数
		MaxConnsPerHost:     50,
		MaxIdleConns:        1000,              // 最大连接数
		IdleConnTimeout:     120 * time.Second, // 空闲连接的最大时间
		DisableKeepAlives:   false,
	}
	http2.ConfigureTransport(probeTransport) // 添加支持HTTP2注入
	// probeClient
	probeClient := &http.Client{
		Transport:     probeTransport,
		Timeout:       30 * time.Second, // 每个请求返回的完整时间限制
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	ClientInfo = append(ClientInfo, &NetCardClientInfo{bytesInterval: 0})

	// 采用一种新的方式，为避免创建过多的client，采用每个client-transport-host单个结构，进行访问的时候进行分开铺展方式
	var ComClients []*http.Client
	for i := 0; i < ClientComNum; i++ {
		TransportCom := AddTransportCom(LocalIP)
		http2.ConfigureTransport(TransportCom)
		CommonClient := &http.Client{
			Transport:     TransportCom,
			Timeout:       0, // 无限时间
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		}
		ComClients = append(ComClients, CommonClient)
		ClientInfo = append(ClientInfo, &NetCardClientInfo{bytesInterval: 0})
	}
	ComAdd := &NetCardHTTPInfoAdd{
		EachClient: ClientInfo,
	}
	// 进行信息的首次注入
	NetCardInfo.mu.Lock()
	NetCardInfo.Content[LocalIP] = ComAdd
	NetCardInfo.mu.Unlock()
	// 收集对应clients
	Clients := &NetCardHTTPClient{
		CommonClient: ComClients,
		ProbeClient:  probeClient,
	}
	NetCardClient.mu.Lock()
	NetCardClient.Content[LocalIP] = Clients
	NetCardClient.mu.Unlock()
}
func AddTransportCom(LocalIP string) *http.Transport {
	CommonTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
			LocalAddr: &net.TCPAddr{
				IP: net.ParseIP(LocalIP),
			},
		}).DialContext,

		TLSClientConfig: &tls.Config{
			NextProtos:         []string{"h2", "http/1.1"},
			ClientSessionCache: tls.NewLRUClientSessionCache(128),
		},
		ForceAttemptHTTP2:   true,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     50,
		MaxIdleConns:        1000,
		IdleConnTimeout:     300 * time.Second,
		DisableKeepAlives:   false,
	}
	return CommonTransport
}

// getProbeClientP 获取对应host根据ms和其他计算出来的Probability获取的client
func (p *NetHTTPCho) getProbeClientP() (string, error) {
	snapshot := p.current.Load()
	if len(snapshot.ProbeEntries) == 0 {
		return "", fmt.Errorf("probe no probability available")
	}
	target := rand.Float64() * snapshot.TotalProbe
	acc := 0.0
	for _, Entry := range snapshot.ProbeEntries {
		acc += Entry.ProbNum
		if target < acc {
			//fmt.Printf("UsedIP: %v\n\n", Entry.IP)
			return Entry.IP, nil
		}
	}
	last := snapshot.ProbeEntries[len(snapshot.ProbeEntries)-1]
	//fmt.Printf("UsedIP: %v\n\n", last.IP)
	return last.IP, nil
}

// PeriodCheck 进行定时的对应相关参数计算
func (p *NetHTTPInfo) PeriodCheck(wg *sync.WaitGroup, MonitorCtx context.Context, MonitorCancel context.CancelFunc) {
	defer wg.Done()
	// 构建dashboard用来进行观测和启动停止
	go StartDashboard(8088, MonitorCancel)
	for {
		select {
		case <-MonitorCtx.Done():
			return
		default:
		}
		var ChunksEntries []ChunksClientEntry
		var ProbeEntries []ProbeClientEntry
		var accChunks float64
		var accProbe float64
		uiCards := make([]UICardInfo, 0)
		Content := p.Content
		p.mu.Lock()
		for LocalIP, CardInfoListAd := range Content {
			// 获取对应bytes
			var SpeedNetCard = 0.0
			var ProbeBytes int64
			var Chunks0Bytes int64
			var Chunks1Bytes int64
			// 使用 Load 方法 (不使用 LoadOrStore，因为只想读)
			if val, ok := NetCardBytes.content.Load(LocalIP); ok {
				// 类型断言
				counters := val.(*ClientBytes)
				// 读取原子值
				ProbeBytes = counters.clientProbeBytes.Load()
				Chunks0Bytes = counters.clientChunks0Bytes.Load()
				Chunks1Bytes = counters.clientChunks1Bytes.Load()
			} else {
				// 如果没找到（还没开始下载），默认为 0
				ProbeBytes = 0
				Chunks0Bytes = 0
				Chunks1Bytes = 0
			}
			//fmt.Printf("ProbeBytes: %v; Chunk0Bytes: %v; Chunk1Bytes: %v\n", ProbeBytes, Chunks0Bytes, Chunks1Bytes)

			ProbeBytesLast := CardInfoListAd.EachClient[0].bytesInterval
			Chunks0BytesLast := CardInfoListAd.EachClient[1].bytesInterval
			Chunks1BytesLast := CardInfoListAd.EachClient[2].bytesInterval
			//fmt.Printf("ProbeBytesLast: %v; Chunk0BytesLast: %v; Chunk1BytesLast: %v\n", ProbeBytesLast, Chunks0BytesLast, Chunks1BytesLast)
			// 计算对应当前速度
			Interval := time.Since(CardInfoListAd.Time).Seconds()
			ProbeSp := float64(ProbeBytes-ProbeBytesLast) / (Interval * (1024 * 1024))
			Chunks0Sp := float64(Chunks0Bytes-Chunks0BytesLast) / (Interval * (1024 * 1024))
			Chunks1Sp := float64(Chunks1Bytes-Chunks1BytesLast) / (Interval * (1024 * 1024))

			// 更新时间（改于25-11-19）
			CardInfoListAd.Time = time.Now()
			CardInfoListAd.EachClient[0].bytesInterval = ProbeBytes
			CardInfoListAd.EachClient[1].bytesInterval = Chunks0Bytes
			CardInfoListAd.EachClient[2].bytesInterval = Chunks1Bytes

			// 计算出最新的有关modelSpeed
			SpeedNetCard = ProbeSp + Chunks0Sp + Chunks1Sp
			alpha := 0.0
			updater := 0.0
			StandardSpeed := CardInfoListAd.StandardSpeed
			lowSpeed := CardInfoListAd.LowAvgSpeed
			var HighSpeed float64
			if CardInfoListAd.FastestSpeed < SpeedNetCard {
				HighSpeed = SpeedNetCard
				CardInfoListAd.FastestSpeed = SpeedNetCard
			} else {
				HighSpeed = CardInfoListAd.FastestSpeed
			}

			if SpeedNetCard > lowSpeed/3 {
				alpha = CheckInterval.Seconds() / (10 * time.Second).Seconds()
				if StandardSpeed > SpeedNetCard {
					if StandardSpeed > 0 {
						x := (StandardSpeed-SpeedNetCard)/StandardSpeed - 1
						updater = (math.Exp(x) - math.Exp(-1)) / (1 - math.Exp(-1))
						StandardSpeed = StandardSpeed + (lowSpeed-StandardSpeed)*alpha*updater
					}
				} else {
					if HighSpeed > StandardSpeed {
						x := (SpeedNetCard-StandardSpeed)/(HighSpeed-StandardSpeed) - 1
						updater = (math.Exp(x) - math.Exp(-1)) / (1 - math.Exp(-1))
						StandardSpeed = StandardSpeed + (HighSpeed-StandardSpeed)*alpha*updater
					}
				}
			}
			// 更新对应CardInfo内容
			CardInfoListAd.StandardSpeed = StandardSpeed
			// 更新ui部分
			uiCards = append(uiCards, UICardInfo{
				IP:            LocalIP,
				StandardSpeed: StandardSpeed,
				NowSpeed:      SpeedNetCard, // 这是你算出来的 SpeedNetCard
				ProbeSpeed:    ProbeSp,      // 实时 Probe 速度
				Chunk0Speed:   Chunks0Sp,    // 实时 Chunk0 速度
				Chunk1Speed:   Chunks1Sp,    // 实时 Chunk1 速度
			})

			// 计算Prob - 添加除零保护
			var ProbeP float64
			var Chunks0P float64
			var Chunks1P float64

			// 辅助函数用于安全计算概率
			calcProb := func(base, speed, divisor float64) float64 {
				if divisor == 0 {
					return base // 如果除数为0，返回基础值
				}
				ratio := 1 - speed/divisor
				return base * ratio * ratio
			}

			if StandardSpeed > SpeedNetCard {
				ProbeP = calcProb(StandardSpeed, ProbeSp, StandardSpeed-Chunks0Sp-Chunks1Sp)
				Chunks0P = calcProb(StandardSpeed, Chunks0Sp, StandardSpeed-Chunks1Sp-ProbeSp)
				Chunks1P = calcProb(StandardSpeed, Chunks1Sp, StandardSpeed-ProbeSp-Chunks0Sp)
			} else {
				ProbeP = calcProb(StandardSpeed, ProbeSp, SpeedNetCard-Chunks0Sp-Chunks1Sp)
				Chunks0P = calcProb(StandardSpeed, Chunks0Sp, SpeedNetCard-Chunks1Sp-ProbeSp)
				Chunks1P = calcProb(StandardSpeed, Chunks1Sp, SpeedNetCard-ProbeSp-Chunks0Sp)
			}

			// 确保概率值非负
			if ProbeP < 0 {
				ProbeP = 0
			}
			if Chunks0P < 0 {
				Chunks0P = 0
			}
			if Chunks1P < 0 {
				Chunks1P = 0
			}

			// 构建snap
			ProbeEntries = append(ProbeEntries, ProbeClientEntry{IP: LocalIP, ProbNum: ProbeP})
			ChunksEntries = append(ChunksEntries, ChunksClientEntry{IP: LocalIP, Index: 0, ProbNum: Chunks0P})
			ChunksEntries = append(ChunksEntries, ChunksClientEntry{IP: LocalIP, Index: 1, ProbNum: Chunks1P})
			accChunks += Chunks0P
			accChunks += Chunks1P
			accProbe += ProbeP
			//fmt.Printf("IP: %s - ProbeSpeed: %v, Chunks0Speed: %v, Chunks1Speed: %v; ProbeP : %v, Chunks0P: %v, Chunks1P: %v \n", LocalIP, ProbeSp, Chunks0Sp, Chunks1Sp, ProbeP, Chunks0P, Chunks1P)
			//fmt.Printf("CardIPSpeed: %v\n", CardInfoListAd.StandardSpeed)
		}

		p.mu.Unlock()

		NewSnapshot := &NetCardHTTPCho{
			ChunksEntries: ChunksEntries,
			TotalChunks:   accChunks,
			ProbeEntries:  ProbeEntries,
			TotalProbe:    accProbe,
		}
		NetCardCho.current.Store(NewSnapshot)

		BroadcastUpdate(uiCards)
		time.Sleep(CheckInterval)
	}
}
