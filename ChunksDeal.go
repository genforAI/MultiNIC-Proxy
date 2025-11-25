package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	ExceedSize  = 100 * 1024 * 1024
	MaxAttempts = 3
	numWorkers  = 5
)

var ContinueOnClientClose = false

// ========== 1. Reader/Writer结构/函数 ==========

type MonitorWriterChunks struct {
	ctx     context.Context
	Writer  *bufio.ReadWriter
	Monitor *ClientBytesRecorder
	LocalIP string
	Index   int
}

func (m *MonitorWriterChunks) Write(p []byte) (n int, err error) {
	select {
	case <-m.ctx.Done():
		return n, m.ctx.Err()
	default:
	}
	n, err = m.Writer.Write(p)
	if m.Index == 0 {
		counters := m.Monitor.GetOrCreate(m.LocalIP)
		counters.clientChunks0Bytes.Add(int64(n))
	} else if m.Index == 1 {
		counters := m.Monitor.GetOrCreate(m.LocalIP)
		counters.clientChunks1Bytes.Add(int64(n))
	}
	return n, err
}

type MonitorReaderChunks struct {
	ctx     context.Context
	Reader  io.Reader
	Monitor *ClientBytesRecorder
	LocalIP string
	Index   int
}

func (m *MonitorReaderChunks) Read(p []byte) (n int, err error) {
	select {
	case <-m.ctx.Done():
		return n, m.ctx.Err()
	default:
	}
	n, err = m.Reader.Read(p)
	if m.Index == 0 {
		counters := m.Monitor.GetOrCreate(m.LocalIP)
		counters.clientChunks0Bytes.Add(int64(n))
	} else if m.Index == 1 {
		counters := m.Monitor.GetOrCreate(m.LocalIP)
		counters.clientChunks1Bytes.Add(int64(n))
	}
	return n, err
}

// ========== 2. 任务结构 ==========

type ChunkTask struct {
	Index       int
	Start       int64
	End         int64
	Attempt     int
	ClientIP    string
	ClientIndex int
}
type ChunkBuffer struct {
	Index int64
	Data  []byte
}
type ChunkResult struct {
	Index int
	Data  []byte
	Err   error
}

// ========== 3. 主函数部分 ==========

func (p *NetHTTPCho) ChunkCalculate(AllSize int64) ([]ChunkTask, error) {
	var chunkTasks []ChunkTask
	BestChunkSizeRecorder.mu.RLock()
	var BestChunkSizeContent = BestChunkSizeRecorder.content
	BestChunkSizeRecorder.mu.RUnlock()
	snapshot := p.current.Load()
	if len(snapshot.ChunksEntries) == 0 {
		return []ChunkTask{}, fmt.Errorf("chunks no probability available")
	}
	fmt.Printf("snapshotChunks: %v\n", snapshot.ChunksEntries)
	var TaskIndex = 0
	var AllStartPos int64 = 0.0
	var AllEndPos int64 = 0.0
	var AllSizePos = AllSize - 1
	var TaskSizePos int64
	for i, Entry := range snapshot.ChunksEntries {
		TaskSize := int64((Entry.ProbNum / snapshot.TotalChunks) * float64(AllSize))
		fmt.Printf("Entry Index%d: TaskSize%d\n", TaskIndex, TaskSize)
		if i == len(snapshot.ChunksEntries)-1 {
			TaskSizePos = AllSizePos
		} else {
			TaskSizePos = AllStartPos + TaskSize - 1
		}
		BestChunk := BestChunkSizeContent[Entry.IP]
		if BestChunk <= 0 {
			BestChunk = 5 * 1024 * 1024
		}
		for {
			if AllStartPos+2*BestChunk-1 <= TaskSizePos {
				AllEndPos += BestChunk - 1
				chunkTasks = append(chunkTasks, ChunkTask{Index: TaskIndex, End: AllEndPos, Start: AllStartPos, ClientIP: Entry.IP, ClientIndex: Entry.Index})
				AllStartPos = AllEndPos + 1
				AllEndPos = AllStartPos
				TaskIndex++
			} else {
				AllEndPos = TaskSizePos
				chunkTasks = append(chunkTasks, ChunkTask{Index: TaskIndex, End: AllEndPos, Start: AllStartPos, ClientIP: Entry.IP, ClientIndex: Entry.Index})
				AllStartPos = AllEndPos + 1
				AllEndPos = AllStartPos
				TaskIndex++
				break
			}
		}
	}
	return chunkTasks, nil
}
func ChunksDeal(bufrw *bufio.ReadWriter, r *http.Request, bag ChunkBag) error {
	// 解析bag内部内容
	AllSize := bag.AllBytes
	TargetURL := bag.TargetURL // Maybe the r.url.string()

	// 计算分块
	TaskChunks, _ := NetCardCho.ChunkCalculate(AllSize)
	fmt.Printf("Total Chunks: %v\n", TaskChunks)
	lenChunks := len(TaskChunks)

	//与客户端解耦
	jobCtx, JobCancel := context.WithCancelCause(context.Background())
	defer JobCancel(nil)
	DirectCtx, DirectCancel := context.WithCancelCause(context.Background())
	defer DirectCancel(nil)

	//监听客户端断开讯号
	go func() {
		select {
		case <-r.Context().Done():
			if !ContinueOnClientClose {
				JobCancel(errors.New("client DisConnected"))
				DirectCancel(errors.New("client DisConnected"))
			}
		case <-jobCtx.Done():
			DirectCancel(errors.New("jobCtx Cancel"))
		}
		fmt.Printf("ctx canceled\n")
		return
	}()

	// 构建任务和结果通道
	taskCh := make(chan ChunkTask, lenChunks)
	resultCh := make(chan ChunkResult, 2*numWorkers)
	// 构建直连形式Worker框架
	TaskSizeDirect := AllSize / numWorkers
	TasksDirect, leftStart, err := ChunksDirectTaskGet(TaskChunks, TaskSizeDirect)
	if err != nil {
		return err
	}

	// 提前注入对应后面剩余任务
	TasksLeft := TaskChunks[leftStart:]
	go func() {
		for _, task := range TasksLeft {
			taskCh <- task
		}
	}()

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	go func() {
		DirectChunksWok(r.Context(), TasksDirect, bufrw, TargetURL, r.Header)
		wg.Done()
		DirectCancel(fmt.Errorf("finished all ChunksDirect"))
	}()

	for i := 1; i < numWorkers; i++ {
		go chunkWorker(jobCtx, i, TargetURL, taskCh, resultCh, r.Header, &wg)
	}
	// 等待所有 Worker 完成后关闭结果队列
	go func() {
		wg.Wait()
		close(resultCh)
		close(taskCh)
		fmt.Println("✅ 所有下载任务完成")
	}()

	// 进行流式输出返回客户端
	// 发送循环
	next := leftStart
	pending := make(map[int][]byte)
	remaining := lenChunks - leftStart

	for {
		select {
		case <-jobCtx.Done():
			// 任务被取消
			err := context.Cause(jobCtx)
			if err != nil {
				fmt.Printf("⚠️ Job canceled: %v\n", err)
			}
			return nil
		case res, ok := <-resultCh:
			if !ok {
				// 所有 worker 退出；若还有未完成块，说明是失败取消
				if remaining > 0 {
					fmt.Printf("❌ 任务未完成但结果通道已关闭，剩余: %d\n", remaining)
				}
				return nil
			}
			if res.Err != nil {
				// 失败的块：决定是否重试
				if taskRetryable(res.Err) && res.Index >= 0 {
					// 增加重试次数，回灌任务
					attempt := TaskChunks[res.Index].Attempt + 1
					TaskChunks[res.Index].Attempt = attempt
					if attempt <= MaxAttempts {
						backoff := time.Duration(math.Pow(2, float64(attempt-1))) * 200 * time.Millisecond
						time.AfterFunc(backoff, func() {
							select {
							case taskCh <- ChunkTask{Index: res.Index, Start: TaskChunks[res.Index].Start, End: TaskChunks[res.Index].End, Attempt: attempt}:
							case <-jobCtx.Done():
							}
						})
						fmt.Printf("🔁 重试 Chunk %d（第 %d 次）\n", res.Index, attempt)
						continue
					}
				}
				// 不可重试或超过次数：取消全局
				JobCancel(fmt.Errorf("fatal: chunk %d failed: %v", res.Index, res.Err))
				return nil
			}
			// 成功：缓存并尝试按序写出
			pending[res.Index] = res.Data
		case <-DirectCtx.Done():
			// 进行写入操作
			data, ok := pending[next]
			if !ok {
				continue
			}
			actualSize := len(data)
			chunkSize := fmt.Sprintf("%x\r\n", actualSize)
			if _, err := bufrw.WriteString(chunkSize); err != nil {
				fmt.Printf("err1: %+v\n", err)
				return nil
			}
			if _, err := bufrw.Write(data); err != nil {
				// 写回失败：客户端已断开
				JobCancel(fmt.Errorf("write failed: %w", err))
				return nil
			}
			if _, err := bufrw.WriteString("\r\n"); err != nil {
				fmt.Printf("err3: %+v\n", err)
				return nil
			}
			if err := bufrw.Flush(); err != nil {
				JobCancel(fmt.Errorf("flush failed: %w", err))
				return nil
			}
			delete(pending, next)
			next++
			remaining--
			if remaining == 0 {
				fmt.Println("✅ 全部发送完成")
				return writeChunkedEnd(bufrw)
			}
		}
	} // 这边的逻辑可以尝试1，全放select；2，下面部分的DirectCtx.Done()放在其他select？
}

// ==========  Worker 函数（关键） ==========
func chunkWorker(
	ctx context.Context,
	workerID int,
	targetURL string,
	taskCh <-chan ChunkTask,
	resultCh chan<- ChunkResult,
	headers http.Header,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-taskCh:
			if (task == ChunkTask{}) && len(taskCh) == 0 {
				return
			}
			data, err := downloadOneChunk(ctx, targetURL, task, headers)
			resultCh <- ChunkResult{Index: task.Index, Data: data, Err: err}
		}
	}
}

// DirectChunksWok ========== 下载前面部分分块保证连接通畅性 ==========
func DirectChunksWok(
	ctx context.Context,
	chunks []ChunkTask,
	bufrw *bufio.ReadWriter,
	targetURL string,
	Headers http.Header,
) {
	TaskNum := len(chunks)
	next := 0
	//fmt.Printf("TaskNum: %d\n", TaskNum)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if next >= TaskNum {
				return
			}
			task := chunks[next]

			NetCardClient.mu.RLock()
			IP := task.ClientIP
			ClientIndex := task.ClientIndex
			clientEntry := NetCardClient.Content[IP]
			if clientEntry == nil || len(clientEntry.CommonClient) <= ClientIndex || clientEntry.CommonClient[ClientIndex] == nil {
				NetCardClient.mu.RUnlock()
				fmt.Printf("Client for IP %s, Index %d does not exist.\n", IP, ClientIndex)
				return
			}
			client := clientEntry.CommonClient[ClientIndex]
			NetCardClient.mu.RUnlock()

			// 设置对应Req
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
			if err != nil {
				return
			}
			// 设置头部
			req.Header = Headers.Clone()
			req.Header = ReqH1ToH2Headers(req.Header)
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", task.Start, task.End))
			fmt.Printf("task: %+v\n", task)
			// 发送指令
			resp, err := client.Do(req)
			if err != nil {
				fmt.Printf("client.Do error: %+v\n", err)
				return
			}
			// 设置bufrw-Copy Writer
			monitorWriter := &MonitorWriterChunks{
				Writer:  bufrw,
				Monitor: NetCardBytes,
				LocalIP: IP,
				ctx:     ctx,
				Index:   ClientIndex,
			}
			// 发送回客户端
			actualSize := resp.ContentLength
			chunkSize := fmt.Sprintf("%x\r\n", actualSize)
			if _, err = bufrw.WriteString(chunkSize); err != nil {
				fmt.Printf("err1: %+v\n", err)
				resp.Body.Close()
				return
			}
			_, err = io.Copy(monitorWriter, resp.Body)
			if err != nil {
				fmt.Printf("err2: %+v\n", err)
				resp.Body.Close()
				return
			}
			resp.Body.Close()
			// 写入结束标记
			if _, err = bufrw.WriteString("\r\n"); err != nil {
				fmt.Printf("err3: %+v\n", err)
				return
			}
			bufrw.Flush()
			next++
		}
	}
}

// ========== 下载单个分块（保持不变） ==========
func downloadOneChunk(ctx context.Context, targetURL string, task ChunkTask, Headers http.Header) ([]byte, error) {

	// 暂时设置为对应probeClient
	IP := task.ClientIP
	ClientIndex := task.ClientIndex
	NetCardClient.mu.RLock()
	clientEntry := NetCardClient.Content[IP]
	if clientEntry == nil || len(clientEntry.CommonClient) <= ClientIndex || clientEntry.CommonClient[ClientIndex] == nil {
		NetCardClient.mu.RUnlock()
		return nil, fmt.Errorf("client for IP %s, Index %d does not exist", IP, ClientIndex)
	}
	client := clientEntry.CommonClient[ClientIndex]
	NetCardClient.mu.RUnlock()

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置头部
	req.Header = Headers.Clone()
	req.Header = ReqH1ToH2Headers(req.Header)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", task.Start, task.End))
	//fmt.Printf("task: %+v\n", task)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	monitorReader := &MonitorReaderChunks{
		Reader:  resp.Body,
		Monitor: NetCardBytes,
		LocalIP: IP,
		ctx:     ctx,
		Index:   ClientIndex,
	}
	need := resp.ContentLength
	buf := make([]byte, need)
	n, err := io.ReadFull(monitorReader, buf)
	return buf[:n], err
}

// ========== 6. 辅助函数 ==========

func taskRetryable(err error) bool {
	return strings.HasPrefix(err.Error(), "retryable:")
}

func writeChunkedEnd(bufrw *bufio.ReadWriter) error {
	_, err := bufrw.WriteString("0\r\n\r\n")
	if err != nil {
		return err
	}
	return bufrw.Flush()
}

// ChunksDirectTaskGet 用来保持前面部分的持续性下载
func ChunksDirectTaskGet(AllChunksTasks []ChunkTask, SizeChunksDD int64) ([]ChunkTask, int, error) {
	var chunksDirect []ChunkTask
	var Add int64 = 0
	for _, chunkTask := range AllChunksTasks {
		chunksDirect = append(chunksDirect, chunkTask)
		Add += (chunkTask.End - chunkTask.Start) - 1
		if Add > SizeChunksDD {
			return chunksDirect, chunkTask.Index + 1, nil
		}
	}
	return nil, 0, fmt.Errorf("error on CHunksDirectTaskGet\n")
}
