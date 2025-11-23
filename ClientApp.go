package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
)

type userConfig struct {
	ListenPort int
	ListenAddr string
}
type UserConfig struct {
	mu      sync.RWMutex
	Content userConfig
}

var GloUserConfig = &UserConfig{
	Content: userConfig{
		ListenAddr: "127.0.0.1",
		ListenPort: 10808,
	},
}

// 本程序用来进行基本初始化，并实现GUI界面方便Windows用户使用
func main() {
	GloUserConfig.mu.RLock()
	Addr := GloUserConfig.Content.ListenAddr
	Port := GloUserConfig.Content.ListenPort
	GloUserConfig.mu.RUnlock()
	AddrPort := fmt.Sprintf("%s:%d", Addr, Port)

	//  构建代理创建/退出
	err := StartSystemProxy(Addr, Port)
	if err != nil {
		fmt.Println("Error starting proxy server ", err)
	}
	defer func() {
		err := EndSystemProxy()
		if err != nil {
			fmt.Println("Error ending proxy server ", err)
		}
	}()
	// 创建根证书
	CrtErr := checkRotSrtGen()
	if CrtErr != nil {
		fmt.Println("Error starting Crt Deal", CrtErr)
	}
	// 将根证书自动安装到用户根目录上面
	if err := InstallCertToSystem(); err != nil {
		fmt.Printf("警告：自动安装证书失败 (请尝试右键以管理员身份运行): %v\n", err)
	}
	//	初始化对应Policy策略
	err = GlobalPolicyManager.LoadPolicies()
	if err != nil {
		fmt.Println("Load Policy Error")
	}

	// 初始化调用网卡测速
	NetCardCLI()

	// 构建按键的关闭协调体
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 加载URI记录大小文件
	URLLoad()
	defer URLSaveLocal()

	// 构建退出方式
	go EndKeyMonitor(cancel)

	// 创建对应等待组进行有关协程执行内容
	var wg sync.WaitGroup
	wg.Add(2)
	go Listener(&wg, AddrPort, ctx)
	go NetCardInfo.PeriodCheck(&wg, ctx, cancel)
	wg.Wait()
	fmt.Println("End......")
}
func EndKeyMonitor(cancel context.CancelFunc) {
	reader := bufio.NewReader(os.Stdin)
	for {
		// 读取一行输入（会阻塞，直到用户按下 Enter）
		input, _ := reader.ReadString('\n')
		// 清理输入字符串，只保留第一个字符并转换为小写
		input = strings.TrimSpace(input)
		if len(input) > 0 {
			char := strings.ToLower(input)[0]
			if char == 'q' {
				fmt.Println("\n[SHUTDOWN] 检测到 'q' 键，触发全局取消...")
				cancel() // 🚨 核心：调用 cancel() 发送关闭信号
				return   // 退出键盘监听 Goroutine
			}
		}
	}
}
