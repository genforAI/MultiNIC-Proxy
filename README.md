[![Go Report Card](https://goreportcard.com/badge/github.com/genforai/multinic-proxy)](https://goreportcard.com/report/github.com/genforai/multinic-proxy)

[🇨🇳 中文文档](./README_CN.md)

# NetBouncer (Multi-NIC High-Performance Proxy)

🚀 **A lightweight Dual-NIC aggregation and download acceleration proxy tool developed in Go.**
*(Note: This project is currently for testing purposes only. Minor bugs may exist when used as a standard proxy.)*

**NetBouncer** is a high-performance local HTTP/2 proxy server. It intelligently utilizes multiple network interfaces on your computer (e.g., Wi-Fi + Ethernet, or Dual Wi-Fi) to significantly improve file download speeds and network stability through dynamic chunking and concurrent transmission technologies.

Unlike traditional "NIC Bonding" software, NetBouncer requires **no virtual drivers**, modifies **no system-level routing tables**, and is a pure application-layer implementation that works out of the box.

> **💡 Note**: The program console displays real-time traffic monitoring. The speed shown on the downloader client represents the "explicit download" speed, while the proxy layer may handle buffering and "implicit download" processing internally. Specific parameters can be adjusted in the source code based on testing requirements.

---

## ✨ Key Features

* **⚡ Dual-NIC Physical Aggregation**
    * Uses low-level `net/http` control to forcibly bind local requests to specific physical network interfaces.
    * Supports Wi-Fi + Ethernet or any dual-NIC combination, maximizing bandwidth utilization through load balancing algorithms.

* **🧠 Smart Probing & Dynamic Chunking**
    * **Probe Mechanism**: Automatically detects if the target resource supports Range Requests (resumable downloads).
    * **Threshold Strategy**: Small files automatically route through the single fastest link; large files trigger the split-chunking engine.
    * **Dynamic Slicing**: Calculates chunk sizes dynamically based on real-time link speed and quality to avoid bottlenecks (the "short board effect").

* **🚀 High-Performance HTTP/2 Concurrency**
    * Full HTTP/2 support with TCP connection reuse to minimize handshake latency.
    * Built-in high-concurrency Worker pool supporting multi-threaded parallel downloads and streaming responses to the client.

* **🔒 Security & Privacy**
    * Built-in mechanism for automatic CA certificate generation and management.
    * Supports transparent proxying and MITM (Man-in-the-Middle) decryption for HTTPS traffic to enable acceleration of encrypted streams.

* **🛠️ Lightweight & Portable**
    * Single-file executable (Go Static Build) with no third-party dependencies.
    * Runs without administrator privileges (except for initial certificate installation); does not modify the system registry or drivers.

---

<h2>🎥 Demos</h2>
<table>
  <tr>
    <td width="50%">
      <div align="center"><b>1. Single NIC - Direct Download</b></div>
      <img src="https://github.com/user-attachments/assets/06967c8b-5c74-42a6-a7de-bcd0b8b266a9" width="100%"/>
    </td>
    <td width="50%">
      <div align="center"><b>2. Single NIC - Proxy Strategy</b></div>
      <img src="https://github.com/user-attachments/assets/4cac5586-d25f-430a-97ea-3fc43558ff15" width="100%"/>
    </td>
  </tr>
  <tr>
    <td width="50%">
      <div align="center"><b>3. Multi-NIC - Direct Download</b></div>
      <img src="https://github.com/user-attachments/assets/6d27675f-25a1-441e-8db2-7b7dd44d61eb" width="100%"/>
    </td>
    <td width="50%">
      <div align="center"><b>4. Multi-NIC - Proxy Strategy</b></div>
      <img src="https://github.com/user-attachments/assets/98cafaea-4613-473a-aff8-23a190d7d163" width="100%"/>
    </td>
  </tr>
</table>

---

## 🛠️ Architecture

<img width="900" height="750" alt="NetBouncer Architecture" src="https://github.com/user-attachments/assets/1e223a90-7008-4ae1-aeae-cd6b3ebf2943" />

1.  **Traffic Hijacking**: The user points the browser or downloader proxy to NetBouncer (Default: `127.0.0.1:10808`).
2.  **Probe**: The proxy intercepts the request and sends a lightweight `HEAD` or small-byte `GET` request first.
3.  **Strategy**:
    * If file size is small (<10MB) -> Direct connection via the lowest latency NIC.
    * If file size is large (>10MB) -> Activate the Chunking Engine.
4.  **Dispatch**:
    * Calculate Chunk Tasks.
    * Assign tasks to HTTP Clients bound to different Source IPs (NICs).
5.  **Reassemble**:
    * Real-time data stream reassembly in memory using Go's `io.Pipe` and `bufio`.
    * Zero-copy write-back to the Client Response Writer.

---

## 📦 Installation & Usage

### Requirements
* **OS**: Windows 10/11 (Recommended), Linux, macOS
* **Hardware**: At least two active network interfaces connected to the internet.

### 1. Download & Run
* Download the latest `NetBouncer.exe` from the [Releases](https://github.com/genforAI/MultiNIC-Proxy/releases) page and run it.

### 2. Certificate Configuration
* Upon the first run, the program will automatically generate a `certs` folder in the current directory and attempt to install the CA certificate into the current user's trust store.
* If the automatic installation succeeds, no action is needed.
* If it fails, please manually double-click `certs/rootCA.crt` to install it into the "Trusted Root Certification Authorities" store.

### 3. Proxy Setup
* Configure your browser (Chrome/Edge) or download manager (IDM) with the following proxy settings:
    * **Protocol**: HTTP / HTTPS
    * **Address**: `127.0.0.1`
    * **Port**: `10808` (Default)

---

## ⚠️ Notes

* **USB NIC Sleep Issue**: If using an external USB network adapter, please disable "Allow the computer to turn off this device to save power" in Device Manager to prevent `i/o timeout` errors during high-concurrency downloads.
* **HTTPS Warning**: Since a self-signed CA is used for traffic acceleration, browsers may display a security warning upon the first visit to an HTTPS site. Please ensure the root certificate is correctly trusted.

## 🛠️ Tech Stack
* **Language**: Golang 1.23+
* **Network**: `net/http`, `golang.org/x/net/http2`
* **Concurrency**: `sync/atomic`, `Goroutines`, `Channels`
* **Crypto**: `crypto/tls`, `crypto/x509`

## 📝 Disclaimer
* This project is intended for **learning and research purposes only** regarding network programming, concurrency control, and proxy technologies.
* Please do not use it for illegal purposes. The developer is not responsible for any data loss or network issues resulting from the use of this software.

---

Copyright © 2025. All Rights Reserved.


