package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

func InstallCertToSystem() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("目前只支持 Windows 系统的自动安装")
	}

	// 证书的通用名称 (Common Name)，必须与 generateCA 中设置的一致
	certName := "Multi-NIC Proxy Root CA"

	// 1. 检查证书是否已存在
	// certutil -user -store Root "Name"
	// 如果找到了证书，命令会返回 exit code 0 (err == nil)
	// 如果没找到，命令会返回 error
	checkCmd := exec.Command("certutil", "-user", "-store", "Root", certName)
	checkCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} // 隐藏窗口

	// 我们不需要关心输出内容，只需要关心命令是否执行成功
	if err := checkCmd.Run(); err == nil {
		fmt.Println("[System] 检测到根证书已存在，跳过安装。")
		return nil
	}

	// 2. 如果上面报错了（说明没找到），则进行安装
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return fmt.Errorf("证书文件不存在: %s", certPath)
	}

	fmt.Println("[System] 未检测到证书，正在安装到当前用户信任库...")

	// 安装命令
	installCmd := exec.Command("certutil", "-user", "-addstore", "Root", certPath)
	installCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := installCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("安装失败: %v, 输出: %s", err, string(output))
	}

	fmt.Printf("[System] 证书安装成功!\n")
	return nil
}
