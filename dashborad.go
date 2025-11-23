package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// --- 1. 数据结构定义 ---

// UIDataPacket 发送给前端的数据包
type UIDataPacket struct {
	Type      string       `json:"type"` // "update" 或 "status"
	Running   bool         `json:"running"`
	Timestamp int64        `json:"timestamp"`
	Cards     []UICardInfo `json:"cards"`
}

type UICardInfo struct {
	IP            string  `json:"ip"`
	StandardSpeed float64 `json:"standard_speed"`
	NowSpeed      float64 `json:"now_speed"`
	// 每个客户端的实时速度
	ProbeSpeed  float64 `json:"probe_speed"`
	Chunk0Speed float64 `json:"chunk0_speed"`
	Chunk1Speed float64 `json:"chunk1_speed"`
}

// --- 2. WebSocket 管理器 ---

var (
	IsSystemRunning = true // 用于控制开始/停止的全局状态（你需要根据你的逻辑连接这个）
	// WebSocket 升级器
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	// 连接池
	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex
	// 广播通道
	broadcast = make(chan UIDataPacket)
	// GlobalCancelFunc 新增：全局变量存储取消函数
	GlobalCancelFunc context.CancelFunc
)

// StartDashboard 启动 Web 服务器
func StartDashboard(port int, cancel context.CancelFunc) {
	GlobalCancelFunc = cancel
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs)
	http.HandleFunc("/api/control", handleControl) // 控制开始/停止

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	fmt.Printf("Dashboard started at http://%s\n", addr)

	// 自动打开浏览器
	go openBrowser("http://" + addr)

	// 启动广播处理协程
	go handleMessages()

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func handleMessages() {
	for packet := range broadcast {
		clientsMu.Lock()
		msg, _ := json.Marshal(packet)
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
		clientsMu.Unlock()
	}
}

// 推送数据给前端的公开方法

func BroadcastUpdate(cards []UICardInfo) {
	packet := UIDataPacket{
		Type:      "update",
		Running:   IsSystemRunning,
		Timestamp: time.Now().UnixMilli(),
		Cards:     cards,
	}
	// 非阻塞发送，防止前端卡死影响后端
	select {
	case broadcast <- packet:
	default:
	}
}

// --- 3. HTTP 处理函数 ---

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	clientsMu.Lock()
	clients[ws] = true
	clientsMu.Unlock()
}

func handleControl(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	if action == "start" {
		// Start 逻辑比较复杂（涉及重新初始化），通常建议重启程序
		// 这里暂时留空或做简单处理
		fmt.Println("System Start Requested (Not Implemented)")
	} else if action == "stop" {
		// 4. 实现停止逻辑：调用全局的 cancel 函数
		if GlobalCancelFunc != nil {
			fmt.Println("UI触发停止：正在关闭系统...")
			GlobalCancelFunc() // 这会触发 context.Done()
		}
	}
	w.WriteHeader(http.StatusOK)
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(htmlContent))
}

// openBrowser 自动打开浏览器
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Println("Could not open browser:", err)
	}
}

// --- 4. 前端代码 (嵌入在 Go 中以便单文件运行) ---
// 使用了 Bootstrap 5 和 ECharts

const htmlContent = `
<!DOCTYPE html>
<html lang="en" data-bs-theme="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Multi-NIC Proxy Dashboard</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <script src="https://cdn.jsdelivr.net/npm/echarts@5.4.3/dist/echarts.min.js"></script>
    <style>
        body { background-color: #121212; color: #e0e0e0; font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; }
        .card { background-color: #1e1e1e; border: 1px solid #333; margin-bottom: 20px; box-shadow: 0 4px 6px rgba(0,0,0,0.3); }
        .card-header { background-color: #252525; border-bottom: 1px solid #333; font-weight: bold; color: #4db8ff; display: flex; justify-content: space-between; }
        .speed-badge { font-size: 0.9em; padding: 5px 10px; border-radius: 4px; background: #333; color: #fff; margin-left: 10px; }
        .chart-container { height: 300px; width: 100%; }
        .status-dot { height: 10px; width: 10px; border-radius: 50%; display: inline-block; margin-right: 5px; }
        .status-on { background-color: #00ff00; box-shadow: 0 0 5px #00ff00; }
        .status-off { background-color: #ff0000; box-shadow: 0 0 5px #ff0000; }
        #control-panel { margin-bottom: 20px; padding: 15px; background: #1e1e1e; border-radius: 8px; border: 1px solid #333; display: flex; align-items: center; justify-content: space-between; }
    </style>
</head>
<body>
    <div class="container mt-4">
        <div id="control-panel">
            <div class="d-flex align-items-center">
                <h3 class="m-0 me-3">🚀 Network Proxy Monitor</h3>
                <div>
                    <span id="status-indicator" class="status-dot status-on"></span>
                    <span id="status-text">Running</span>
                </div>
            </div>
            <div>
                <button class="btn btn-success me-2" onclick="controlSystem('start')">▶ Start</button>
                <button class="btn btn-danger" onclick="controlSystem('stop')">⏹ Stop</button>
            </div>
        </div>

        <div id="cards-container" class="row">
        </div>
    </div>

    <script>
        const container = document.getElementById('cards-container');
        const charts = {}; 
        let isRunning = true;

        const ws = new WebSocket('ws://' + window.location.host + '/ws');

        ws.onopen = function() { console.log("WebSocket Connected"); };

        ws.onmessage = function(event) {
            try {
                const data = JSON.parse(event.data);
                if (data.type === 'update') {
                    updateStatus(data.running);
                    updateDashboard(data.timestamp, data.cards);
                }
            } catch (e) {
                console.error("Error parsing WS data:", e);
            }
        };

        ws.onerror = function(error) { console.error("WebSocket Error:", error); };

        function updateStatus(running) {
            isRunning = running;
            const dot = document.getElementById('status-indicator');
            const text = document.getElementById('status-text');
            if (running) {
                dot.className = 'status-dot status-on';
                text.innerText = 'Running';
                text.style.color = '#00ff00';
            } else {
                dot.className = 'status-dot status-off';
                text.innerText = 'Stopped';
                text.style.color = '#ff4444';
            }
        }

        function controlSystem(action) {
            fetch('/api/control?action=' + action)
                .then(response => console.log("Control action sent:", action))
                .catch(error => console.error("Control error:", error));
        }

        // 核心更新逻辑
        function updateDashboard(timestamp, cards) {
            // 计算窗口范围：[当前 - 60s, 当前]
            const windowSize = 60 * 1000; 
            const minTime = timestamp - windowSize;
            const maxTime = timestamp;

            if (!cards) return;

            cards.forEach(function(card) {
                var cardElem = document.getElementById('card-' + card.ip);
                if (!cardElem) {
                    createCard(card);
                    cardElem = document.getElementById('card-' + card.ip);
                }

                // 更新数字
                var stdElem = document.getElementById('std-' + card.ip);
                var nowElem = document.getElementById('now-' + card.ip);
                if (stdElem) stdElem.innerText = card.standard_speed.toFixed(2) + ' MB/s';
                if (nowElem) nowElem.innerText = card.now_speed.toFixed(2) + ' MB/s';

                // 更新图表
                if (charts[card.ip]) {
                    const chart = charts[card.ip];
                    const option = chart.getOption();
                    
                    // 构造数据点：[时间戳, 数值]
                    // ECharts time 轴会自动处理时间戳
                    const newStd = { name: timestamp, value: [timestamp, card.standard_speed] };
                    const newProbe = { name: timestamp, value: [timestamp, card.probe_speed] };
                    const newChunk0 = { name: timestamp, value: [timestamp, card.chunk0_speed] };
                    const newChunk1 = { name: timestamp, value: [timestamp, card.chunk1_speed] };

                    // 添加新数据
                    option.series[0].data.push(newStd);
                    option.series[1].data.push(newProbe);
                    option.series[2].data.push(newChunk0);
                    option.series[3].data.push(newChunk1);

                    // 清理旧数据 (早于 minTime 的)
                    // 只需检查 series[0]，其他同步清理
                    while (option.series[0].data.length > 0) {
                        // data[0].value[0] 是 X 轴的时间戳
                        if (option.series[0].data[0].value[0] < minTime) {
                            option.series[0].data.shift();
                            option.series[1].data.shift();
                            option.series[2].data.shift();
                            option.series[3].data.shift();
                        } else {
                            break;
                        }
                    }

                    // 【关键】动态更新 X 轴的范围，实现平滑滑动
                    option.xAxis[0].min = minTime;
                    option.xAxis[0].max = maxTime;

                    chart.setOption(option);
                }
            });
        }

        function createCard(card) {
            var html = 
            '<div class="col-md-6 col-lg-12">' +
                '<div class="card" id="card-' + card.ip + '">' +
                    '<div class="card-header">' +
                        '<span>📡 NIC: ' + card.ip + '</span>' +
                        '<div>' +
                            '<span class="speed-badge" style="border: 1px solid #808080; color: #a0a0a0;">Std: <span id="std-' + card.ip + '">0</span></span>' +
                            '<span class="speed-badge" style="border: 1px solid #00ff00; color: #00ff00;">Now: <span id="now-' + card.ip + '">0</span></span>' +
                        '</div>' +
                    '</div>' +
                    '<div class="card-body">' +
                        '<div id="chart-' + card.ip + '" class="chart-container"></div>' +
                    '</div>' +
                '</div>' +
            '</div>';
            
            container.insertAdjacentHTML('beforeend', html);
            initChart(card.ip);
        }

        function initChart(ip) {
            const chartDom = document.getElementById('chart-' + ip);
            const myChart = echarts.init(chartDom, 'dark', {renderer: 'canvas'});
            
            const option = {
                backgroundColor: 'transparent',
                tooltip: { 
                    trigger: 'axis',
                    // 格式化 tooltip 显示时间
                    formatter: function (params) {
                        if (!params.length) return '';
                        const date = new Date(params[0].value[0]);
                        let html = date.toTimeString().split(' ')[0] + '<br/>';
                        params.forEach(item => {
                            html += item.marker + item.seriesName + ': ' + item.value[1].toFixed(2) + ' MB/s<br/>';
                        });
                        return html;
                    }
                },
                legend: { data: ['Capacity', 'Probe', 'Chunk 0', 'Chunk 1'], bottom: 0 },
                grid: { top: 30, left: 50, right: 20, bottom: 40 },
                // 【关键】X 轴改为时间类型
                xAxis: { 
                    type: 'time', 
                    splitLine: { show: false },
                    axisLabel: {
                        formatter: function (value) {
                            // 格式化 X 轴时间标签
                            return new Date(value).toTimeString().split(' ')[0];
                        }
                    }
                },
                yAxis: { type: 'value', name: 'MB/s', splitLine: { lineStyle: { color: '#333' } } },
                series: [
                    { 
                        name: 'Capacity', 
                        type: 'line', 
                        showSymbol: false, 
                        data: [], 
                        lineStyle: { width: 2, color: '#808080', type: 'dashed' },
                        areaStyle: { opacity: 0.1, color: '#808080' },
                        z: 1 
                    },
                    { name: 'Probe', type: 'line', smooth: true, showSymbol: false, data: [], lineStyle: { width: 2, color: '#ff00ff' } },
                    { name: 'Chunk 0', type: 'line', smooth: true, showSymbol: false, data: [], lineStyle: { width: 2, color: '#00ccff' } },
                    { name: 'Chunk 1', type: 'line', smooth: true, showSymbol: false, data: [], lineStyle: { width: 2, color: '#ffff00' } }
                ],
                animation: false // 关闭动画以减少 CPU 消耗，平滑滚动
            };
            myChart.setOption(option);
            charts[ip] = myChart;

            window.addEventListener('resize', function() {
                myChart.resize();
            });
        }
    </script>
</body>
</html>
`
