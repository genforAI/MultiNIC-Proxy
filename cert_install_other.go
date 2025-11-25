//go:build !windows
// +build !windows

package main

import (
	"fmt"
	"runtime"
)

func InstallCertToSystem() error {
	return fmt.Errorf("自动证书安装目前只支持 Windows 系统，当前系统: %s", runtime.GOOS)
}
