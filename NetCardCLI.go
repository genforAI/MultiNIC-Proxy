package main

var BestProbeClientIP string

func NetCardCLI() {

	// 默认测试参数加载
	NetworkTester.DefaultTestConfig()
	// 网卡参数加载
	InitNetCardInfo()
	// Client绑定性加载 以及 Cho的基本初始化
	IPHTTPClientAndChoInit()
	// 进行TCP测试
	NetworkTester.TCPSpeedTest()
	// 获取专门用来进行Probe部分
	BestProbeClientIP = NetCardInfo.InitBestProbeClient()
	return
}
