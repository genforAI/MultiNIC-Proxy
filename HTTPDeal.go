package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Writer

type MonitorWriterProbe struct {
	ctx     context.Context
	Writer  http.ResponseWriter
	Monitor *ClientBytesRecorder
	LocalIP string
	bytes   atomic.Int64
	flusher http.Flusher
	ifFlush bool
}

func (m *MonitorWriterProbe) Write(p []byte) (n int, err error) {
	// 处理上级出现的ctx情况
	select {
	case <-m.ctx.Done():
		return n, m.ctx.Err()
	default:
	}
	n, err = m.Writer.Write(p)
	counters := m.Monitor.GetOrCreate(m.LocalIP)
	counters.clientProbeBytes.Add(int64(n))
	
	// 这样子保证只进行一次相关的处理优先性加载策略
	if m.ifFlush != true {
		m.bytes.Add(int64(n))
		byteGet := m.bytes.Load()
		if byteGet >= 1*1024*1024 { // 用来针对小资源的快速刷新，如果后面测试无用会去掉
			fmt.Printf("1MB flush")
			m.flusher.Flush()
			m.ifFlush = true
		}
	}
	return n, err
}

func HttpsHandle(tlsConn net.Conn) {
	// 构建请求原子计数器
	var ReqNum atomic.Int64
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandReq(w, r, &ReqNum)
	}) // ?疑问，这个不就是Monitor里面的client里面设计的
	server := &http.Server{
		Handler:      handler,
		ReadTimeout:  30 * time.Second, // 读取请求超时时间
		WriteTimeout: 30 * time.Second, // 发送请求超时时间
		IdleTimeout:  90 * time.Second, // 空闲连接超时时间
		BaseContext:  nil,
		//func(net.Listener) context.Context {
		//	return context.Background()
		//}
	}
	listener := newSingleConnListener(tlsConn)
	err := server.Serve(listener)
	if err != nil {
		fmt.Printf("server serve err:%s\n", err.Error())
	}
	defer tlsConn.Close()
}

type singleConnListener struct {
	conn net.Conn
	once sync.Once
	done chan struct{}
}

func newSingleConnListener(conn net.Conn) *singleConnListener {
	return &singleConnListener{
		conn: conn,
		done: make(chan struct{}),
	}
}
func (l *singleConnListener) Accept() (net.Conn, error) {
	var conn net.Conn
	l.once.Do(func() {
		conn = l.conn
	})
	if conn != nil {
		return conn, nil
	}
	<-l.done // <- 阻塞行为，通道左边，表示接收数据后才会进行行动（关键）：阻塞原因{通道中没有传入的值}
	return nil, net.ErrClosed
}
func (l *singleConnListener) Close() error {
	close(l.done)
	return nil
}
func (l *singleConnListener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

type ChunkBag struct {
	TargetURL string
	AllBytes  int64
	stateCode int64
}

func HandReq(w http.ResponseWriter, r *http.Request, ReqNum *atomic.Int64) {
	defer func() {
		if r.Body != nil {
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
		}
	}()
	var TargetURL string
	//var ifChunks bool
	//var bag ChunkBag
	var err error
	ctx := r.Context()
	TargetURL = "https://" + r.Host + r.RequestURI
	// 计算连接数
	//ReqNum.Add(1)
	//defer ReqNum.Add(-1)
	//reqNums := ReqNum.Load()
	//fmt.Printf("ReqNums:%v\n", reqNums)
	//  进行有关访问和计算
	err = NetCardClient.ProbeFile(ctx, TargetURL, r, w)
	if err != nil {
		fmt.Printf("error happened: %v\n", err)
		return
	}
	//if ifChunks {
	//
	//}
	return
}
func (p *NetHTTPClient) ProbeFile(ctx context.Context, targetURL string, r *http.Request, w http.ResponseWriter) (err error) {
	// 基础参数设置
	var bag ChunkBag
	var ifChunks bool

	// 获取最优IP
	IP, err := NetCardCho.getProbeClientP()
	if err != nil {
		return err
	}
	// 设置为对应probeClient
	NetCardClient.mu.RLock()
	client := NetCardClient.Content[IP].ProbeClient
	if client == nil {
		fmt.Printf("Client no exist.")
		return
	} else {
		//fmt.Printf("client exist: %+v \n", client)
	}
	NetCardClient.mu.RUnlock()

	// 初始情况设置
	bag.AllBytes = -1
	bag.TargetURL = targetURL
	var body io.ReadCloser
	if r.Method != "HEAD" {
		body = r.Body
	} else {
		body = nil
	}
	upStreamReq, err := http.NewRequestWithContext(ctx, r.Method, targetURL, body)

	// 头部处理策略 - 发送到上游时需要删除的headers
	upStreamReq.Header = r.Header.Clone() // 快速拷贝
	upStreamReq.Header = ReqH1ToH2Headers(upStreamReq.Header)

	// 获取resp
	resp, err := client.Do(upStreamReq)
	if err != nil {
		select {
		case <-ctx.Done():
			fmt.Printf("⚠️ 客户端在请求期间断开: %v\n", ctx.Err())
			return err
		default:
		}
		return err
	}
	defer func() {
		resp.Body.Close()
	}()

	// Client Range
	//fmt.Printf("客户端请求Range: %s\n", r.Header.Get("Range"))
	//处理resp，如果是chunks，即可返回
	bag.TargetURL = resp.Request.URL.String() // 如果后面发现无用的话，会将这个优化掉
	ifFound, URLSize, stateCode := URLCheck(targetURL)
	// 优先进行对应查找哈希表(very fast)
	if URLSize >= ExceedSize {
		ifChunks = true
		bag.AllBytes = URLSize
		bag.stateCode = stateCode
		// fmt.Printf("Record-statusCode: %d\n", stateCode)
		fmt.Printf("Chunks Deal Start！\n")
		chunksProbe(w, r, bag)
		return nil
	} else if URLSize == -2 { // 针对一些网站本身就是未知大小的情况进行区分
		var fileSize int64
		var fileCode int64
		ifChunks, fileSize, fileCode, err = RespDeal(resp.Header, resp.StatusCode)
		if err == nil {
			bag.AllBytes = fileSize
			bag.stateCode = fileCode
			URLSave(targetURL, fileCode, fileSize)
			// fmt.Printf("Search-statusCode: %d\n", fileCode)
			if ifChunks == true {
				fmt.Printf("Chunks Deal Start！\n")
				chunksProbe(w, r, bag)
				return nil
			}
		}
	} else if URLSize == 0 && stateCode == 200 {
		if !ifFound {
			URLSave(targetURL, 200, 0)
		}
	}

	// 如果发现需要进行分块加速处理的话，首先预热一下对应的host，然后先发送头和部分数据并将已经下载的already记录下来，进行分块处理
	// 创建用于记录的Monitor
	// 创建对应w的http.Flusher
	var flusher http.Flusher
	if _, ok := w.(http.Flusher); ok {
		flusher = w.(http.Flusher)
	}
	monitorWriter := &MonitorWriterProbe{
		Writer:  w,
		Monitor: NetCardBytes,
		LocalIP: IP,
		ctx:     ctx,
		flusher: flusher,
	}
	//resp.Header.Set("Content-Length", strconv.FormatInt(URLSize, 10)) 存在问题，对于部分文件大小未经过探测到的
	resp.Header.Set("Accept-Ranges", "bytes")
	resp.Header.Del("Content-Range")
	resp.Header.Del("Transfer-Encoding")
	resp.Header.Del("Connection")
	// 复制响应headers到客户端，同时删除不应该传递的headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode) // 这里最好还是别用stateCode，避免出现-1的情况。客户端自行进行处理。
	monitorWriter.ifFlush = false
	_, err = io.Copy(monitorWriter, resp.Body)
	if err != nil {
		select {
		case <-ctx.Done():
			return fmt.Errorf("ctx Done: %v\n", ctx.Err())
		default:
			fmt.Println(err)
			return err
		}
	}
	return nil
}

// 删除发送到上游服务器时不应该包含的headers

func ReqH1ToH2Headers(reqHead http.Header) http.Header {
	// 删除 hop-by-hop headers (RFC 7230)
	h := reqHead
	h.Del("Connection")
	h.Del("Keep-Alive")
	h.Del("Proxy-Authenticate")
	h.Del("Proxy-Authorization")
	h.Del("Proxy-Connection") // 非标准但常见
	h.Del("Te")
	h.Del("Trailers")
	h.Del("Transfer-Encoding")
	h.Del("Upgrade")
	//h.Set("User-Agent", h.Get("User-Agent")+"H2-Proxy/1.0") // ?
	h.Set("Accept-Encoding", "identity")

	// 删除可能影响代理行为的headers
	//h.Del("Accept-Encoding") // 让上游服务器决定编码
	return h
}
func chunksProbe(w http.ResponseWriter, r *http.Request, bag ChunkBag) {
	if bag.stateCode == http.StatusPartialContent {
		fmt.Printf("statuCode: %d\n", bag.stateCode)
		return
	}
	if bag.stateCode == http.StatusOK {
		conn, bufrw, err := Hijack(w, bag.AllBytes)
		if err != nil {
			fmt.Printf("Hijack Error: %v\n", err)
			conn.Close()
			return
		}
		err = ChunksDeal(bufrw, r, bag)
		if err != nil {
			fmt.Printf("ChunksDeal Error: %v\n", err)
			conn.Close()
			return
		}
		conn.Close()
		return
	}
	fmt.Printf("Not 206 or 200 , statuCode: %d\n", bag.stateCode)
	return
}
