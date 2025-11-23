package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// URLSave

type GlobalFileSizeCache struct {
	Cache    sync.Map
	FilePath string
}
type GlobalCodeCache struct {
	Cache    sync.Map
	FilePath string
}

var GloFileSizeCache = &GlobalFileSizeCache{
	FilePath: "./Cache/URIFileSize.json",
}
var GloCodeCache = &GlobalCodeCache{
	FilePath: "./Cache/URICode.json",
}

func (p *GlobalFileSizeCache) URLSizeLoad() error {
	if _, err := os.Stat(p.FilePath); os.IsNotExist(err) {
		fmt.Printf("⚠️ 缓存文件不存在: %s，使用空缓存\n", p.FilePath)
		return nil
	}
	data, err := os.ReadFile(p.FilePath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %v", err)
	}
	tmp := make(map[string]int64)
	if err := json.Unmarshal(data, &tmp); err != nil {
		return fmt.Errorf("JSON 解析失败: %v", err)
	}
	// 清空旧数据（若需要）并填充
	p.Cache.Range(func(key, value any) bool {
		p.Cache.Delete(key)
		return true
	})
	for k, v := range tmp {
		p.Cache.Store(k, v)
	}
	fmt.Printf("✅ 缓存FileSize已加载: %d 条记录\n", len(tmp))
	return nil
}
func (p *GlobalCodeCache) URLCodeLoad() error {
	if _, err := os.Stat(p.FilePath); os.IsNotExist(err) {
		fmt.Printf("⚠️ 缓存文件不存在: %s，使用空缓存\n", p.FilePath)
		return nil
	}
	data, err := os.ReadFile(p.FilePath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %v", err)
	}
	tmp := make(map[string]int64)
	if err := json.Unmarshal(data, &tmp); err != nil {
		return fmt.Errorf("JSON 解析失败: %v", err)
	}
	// 清空旧数据（若需要）并填充
	p.Cache.Range(func(key, value any) bool {
		p.Cache.Delete(key)
		return true
	})
	for k, v := range tmp {
		p.Cache.Store(k, v)
	}
	fmt.Printf("✅ 缓存FileSize已加载: %d 条记录\n", len(tmp))
	return nil
}
func URLLoad() {
	GloFileSizeCache.URLSizeLoad()
	GloCodeCache.URLCodeLoad()
}

func (p *GlobalFileSizeCache) URLFileSizeSaveLocal() {
	tmp := make(map[string]int64)
	p.Cache.Range(func(k, v any) bool {
		ks, ok := k.(string)
		if !ok {
			return true
		}
		if vi, ok := v.(int64); ok {
			tmp[ks] = vi
		}
		return true
	})

	dir := filepath.Dir(p.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Printf("创建目录失败: %v", err)
	}
	data, err := json.MarshalIndent(tmp, "", "  ")
	if err != nil {
		fmt.Printf("序列化失败: %v\n", err)
	}
	if err := os.WriteFile(p.FilePath, data, 0o644); err != nil {
		fmt.Printf("写入文件失败: %v\n", err)
	}
	fmt.Printf("缓存已经保存到本地目录\n")
}

func (p *GlobalCodeCache) URLCodeSaveLocal() {
	tmp := make(map[string]int64)
	p.Cache.Range(func(k, v any) bool {
		ks, ok := k.(string)
		if !ok {
			return true
		}
		if vi, ok := v.(int64); ok {
			tmp[ks] = vi
		}
		return true
	})

	dir := filepath.Dir(p.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Printf("创建目录失败: %v", err)
	}
	data, err := json.MarshalIndent(tmp, "", "  ")
	if err != nil {
		fmt.Printf("序列化失败: %v\n", err)
	}
	if err := os.WriteFile(p.FilePath, data, 0o644); err != nil {
		fmt.Printf("写入文件失败: %v\n", err)
	}
	fmt.Printf("缓存已经保存到本地目录\n")
}
func URLSaveLocal() {
	GloCodeCache.URLCodeSaveLocal()
	GloFileSizeCache.URLFileSizeSaveLocal()
}

// URLSave 存记录
func (p *GlobalFileSizeCache) URLSave(url string, fileSize int64) {
	p.Cache.Store(url, fileSize)
}
func (p *GlobalCodeCache) URLSave(url string, urlCode int64) {
	p.Cache.Store(url, urlCode)
}
func URLSave(url string, urlCode int64, urlSize int64) {
	GloFileSizeCache.URLSave(url, urlSize)
	GloCodeCache.URLSave(url, urlCode)
}

// URLCheck ：如果有对应的URL记录，则判断为不用进行对应探测，如果无对应URL，则需要进行网页深度搜索
func URLCheck(url string) (bool, int64, int64) {
	var urlCode int64
	var urlSize int64
	// 查找对应url code/size
	if v, ok := GloFileSizeCache.Cache.Load(url); ok {
		if size, ok2 := v.(int64); ok2 {
			// fmt.Println("找到对应保存URL匹配")
			urlSize = size
			if vv, okk := GloCodeCache.Cache.Load(url); okk {
				if code, okk2 := vv.(int64); okk2 {
					// fmt.Println("找到对应保存URL匹配")
					urlCode = code
				}
			}
		}
		return true, urlSize, urlCode
	}
	lowerURI := strings.ToLower(url)
	smallExtensions := []string{".js", ".css", ".jsx", ".gif", ".ico", ".svg", ".ms4", ".ts", ".m3u8", "mpd"}
	for _, ext := range smallExtensions {
		if strings.HasSuffix(lowerURI, ext) {
			// fmt.Printf("发现为小型文件自动返回0")
			return false, 0, 200
		}
	}
	// fmt.Println("未找到对应URL")
	return false, -2, -2
}
