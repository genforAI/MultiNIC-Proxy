```batch
@echo off
if exist build rmdir /s /q build
mkdir build
mkdir build\certs

:: 设置环境变量，确保生成的是纯静态文件，不依赖系统DLL
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64

:: -ldflags "-s -w" 可以去掉调试符号，让exe体积更小
go build -ldflags "-s -w" -o build/NetBouncer.exe .

if %errorlevel% neq 0 (
    pause
    exit /b
)

if exist README.md copy README.md build\

pause