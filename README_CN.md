(https://goreportcard.com/badge/github.com/genforai/multinic-proxy)](https://goreportcard.com/report/github.com/genforai/multinic-proxy)

[🇺🇸 English](./README.md)
# NetBouncer (Multi-NIC High-Performance Proxy)

🚀 **基于 Go 语言开发的轻量级双网卡聚合下载加速代理工具（仅用于测试，在正常作为代理使用时存在部分bug未作调整）**

NetBouncer 是一个高性能的本地 HTTP/2 代理服务器。它能够智能利用计算机上的多个网络接口（如 Wi-Fi + 有线网卡，或双 Wi-Fi），通过动态分片和多路并发技术，显著提升文件的下载速度和网络稳定性。

与传统的“网卡绑定”软件不同，NetBouncer 不需要安装任何虚拟驱动，无需修改系统底层路由，纯应用层实现，即插即用。

> **💡 说明**：程序控制台会实时显示总流量监控。下载器客户端显示的部分为“显式下载”速度，代理层内部可能存在缓冲等“隐式下载”处理，具体参数可在源代码中根据测试需求进行调整。

** 演示示例：

---

## ✨ 核心特性 (Key Features)

* **⚡ 双网卡物理聚合 (Dual-NIC Aggregation)**
    * 通过 `net/http` 底层控制，强制绑定本地请求到指定的物理网卡出口。
    * 支持 WiFi + Ethernet 或任意双网卡同时工作，通过负载均衡算法最大化带宽利用率。

* **🧠 智能探测与动态分片 (Smart Chunking)**
    * **Probe 机制**：自动探测目标资源是否支持断点续传（Range Request）。
    * **大小阈值判断**：小文件自动走最优单链路，大文件自动触发分片加速。
    * **动态切片**：根据网卡实时速度和连接质量，动态计算分片大小，拒绝“木桶效应”。

* **🚀 HTTP/2 高性能并发**
    * 完全支持 HTTP/2 协议，复用 TCP 连接，减少握手延迟。
    * 内置高并发 Worker 池，支持多线程并行下载并流式回传给客户端。

* **🔒 安全与隐私**
    * 内置 CA 证书自动生成与管理机制。
    * 支持 HTTPS 流量的透明代理与解密（MITM），实现对加密流量的加速处理。

* **🛠️ 轻量级与便携**
    * 单文件运行（Go Static Build），无第三方依赖。
    * 无需管理员权限即可运行（证书安装除外），不修改系统注册表或驱动。

---

<h2>🎥 效果演示 (Demos)</h2>
<table>
  <tr>
    <td width="50%">
      <div align="center"><b>1. 单网卡原始下载</b></div>
      <img src="https://github.com/user-attachments/assets/6d27675f-25a1-441e-8db2-7b7dd44d61eb" width="100%"/>
    </td>
    <td width="50%">
      <div align="center"><b>2. 单网卡代理策略下载</b></div>
      <img src="https://github.com/user-attachments/assets/4cac5586-d25f-430a-97ea-3fc43558ff15" width="100%"/>
    </td>
  </tr>
  <tr>
    <td width="50%">
      <div align="center"><b>3. 多网卡原始下载</b></div>
      <img src="https://github.com/user-attachments/assets/06967c8b-5c74-42a6-a7de-bcd0b8b266a9" width="100%"/>
    </td>
    <td width="50%">
      <div align="center"><b>4. 多网卡代理策略下载</b></div>
      <img src="https://github.com/user-attachments/assets/98cafaea-4613-473a-aff8-23a190d7d163" width="100%"/>
    </td>
  </tr>
</table>

---

## 🛠️ 架构原理 (Architecture)

1.  **流量劫持**：用户将浏览器或下载器的代理指向 NetBouncer (默认 `127.0.0.1:8088`)。
2.  **探测 (Probe)**：代理服务器拦截请求，先发起一次轻量级的 `HEAD` 或小字节 `GET` 请求。
3.  **决策 (Strategy)**：
    * 如果文件较小 (<10MB) -> 使用当前延迟最低的网卡直连。
    * 如果文件较大 (>10MB) -> 启动分片引擎。
4.  **分发 (Dispatch)**：
    * 计算分片任务（Chunk Tasks）。
    * 将任务分配给绑定了不同 Source IP（网卡）的 HTTP Client。
5.  **重组 (Reassemble)**：
    * 利用 Go 的 `io.Pipe` 和 `bufio` 在内存中实时重组数据流。
    * 零拷贝直接写回客户端 Response Writer。

---

## 📦 安装与使用 (Installation)

### 环境要求
* 操作系统：Windows 10/11 (推荐), Linux, macOS
* 硬件：拥有至少两个可用的网络接口（且已连接互联网）

### 1. 下载与运行
* 下载最新版本的 `NetBouncer.exe`，直接双击运行。

### 2. 证书配置
* 首次运行时，程序会自动在当前目录生成 certs 文件夹，并尝试将 CA 证书安装到当前用户的信任列表中。
* 如果自动安装成功，无需操作。
* 如果失败，请手动双击 certs/rootCA.crt 安装到“受信任的根证书颁发机构”。
* 
### 3. 设置代理
* 配置你的浏览器（Chrome/Edge）或下载软件（IDM）的代理服务器设置：
* 协议: HTTP / HTTPS
* 地址: 127.0.0.1
* 端口: 10808 (默认)

--- 

## ⚠️ 注意事项 (Notes)
* USB 网卡休眠问题：如果你使用外置 USB 网卡，请在设备管理器中关闭“允许计算机关闭此设备以节约电源”选项，否则高并发下载时可能会出现 i/o timeout。
* HTTPS 警告：由于使用了自签名 CA 进行流量加速，初次访问 HTTPS 网站时浏览器可能会提示安全警告，请确保根证书已正确信任。

## 🛠️ 技术栈 (Tech Stack)
* Language: Golang 1.23+
* Network: net/http, golang.org/x/net/http2
* Concurrency: sync/atomic, Goroutines, Channels
* Crypto: crypto/tls, crypto/x509

## 📝 免责声明 (Disclaimer)
* 本项目仅供学习和研究网络编程、并发控制及代理技术测试使用。请勿用于非法用途。开发者不对因使用本软件产生的任何数据丢失或网络问题负责。

---


Copyright © 2025. All Rights Reserved.






