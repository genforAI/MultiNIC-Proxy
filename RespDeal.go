package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func RespDeal(Header http.Header, StatusCode int) (ifChunks bool, fileSize int64, fileCode int64, err error) {
	switch StatusCode {
	case http.StatusPartialContent: // 206
		cr := Header.Get("Content-Range") // 例: "bytes 0-0/207322416" 或 "bytes 0-0/*"
		if partSize, _, ok := parseContentRangeTotal(cr); ok {
			//fmt.Printf("Are you bibi?\n")
			fileSize = partSize
			fileCode = http.StatusPartialContent
			return false, fileSize, fileCode, nil
		} else {
			return false, -1, -1, fmt.Errorf("206 response but invalid Content-Range: %s", cr)
		}
	case http.StatusRequestedRangeNotSatisfiable: // 416
		// 常见返回 "bytes */207322416" —— 当前 Range 越界，但能给出总大小
		cr := Header.Get("Content-Range")
		_, totalSize, _ := parseContentRangeTotal(cr)
		fileSize = totalSize
		// 常见返回 "bytes */207322416" —— 当前 Range 越界，但能给出总大小
		fmt.Printf("❌ Range越界: %s\n", cr)
		return false, fileSize, fileCode, fmt.Errorf("range not satisfiable: %s", cr)

	case http.StatusOK: // 200：这次没按范围返回
		contentLength := Header.Get("Content-Length")
		acceptRanges := Header.Get("Accept-Ranges")
		if cl := headerInt(contentLength); cl >= 0 {
			fileSize = cl
			fileCode = http.StatusOK
			if fileSize >= ExceedSize && acceptRanges == "bytes" { // 这里还要判断能否进行range?
				//fmt.Printf("Are you OK?\n")
				return true, fileSize, fileCode, nil
			} else {
				return false, fileSize, fileCode, nil
			}
		}
		return false, -1, http.StatusOK, nil
	default:
		return false, -1, -1, fmt.Errorf("unkonwn status code: %d", StatusCode)
	}
}

// 统一解析 Content-Range 的 total：
// 兼容 "bytes 0-0/207322416"（206）与 "bytes */207322416"（416）
func parseContentRangeTotal(cr string) (partSize int64, totalSize int64, ok bool) {
	cr = strings.TrimSpace(cr) // 去掉空格换行
	if cr == "" {
		return -1, -1, false
	}
	lc := strings.ToLower(cr) // 判断是否由byte
	if !strings.HasPrefix(lc, "byte") {
		return -1, -1, false
	}
	// 去掉 byte
	rangeStr := strings.TrimSpace(cr[6:])

	slashPos := strings.LastIndexByte(rangeStr, '/')
	if slashPos < 0 {
		return -1, -1, false
	}
	rangePart := strings.TrimSpace(rangeStr[:slashPos])
	totalPart := strings.TrimSpace(rangeStr[slashPos+1:])

	// 解析总文件大小
	totalSize = -1
	if totalPart != "*" {
		if total, err := strconv.ParseInt(totalPart, 10, 64); err == nil && total >= 0 {
			totalSize = total
		}
	}
	// 解析range部分: "36700160-41943039"
	dashPos := strings.IndexByte(rangePart, '-')
	if dashPos < 0 {
		return -1, totalSize, false
	}
	startStr := strings.TrimSpace(rangePart[:dashPos]) // "36700160"
	endStr := strings.TrimSpace(rangePart[dashPos+1:]) // "41943039"
	start, err1 := strconv.ParseInt(startStr, 10, 64)
	end, err2 := strconv.ParseInt(endStr, 10, 64)
	if err1 != nil || err2 != nil || start > end {
		return -1, totalSize, false
	}

	// 计算实际接收的数据大小
	actualSize := end - start + 1 // 41943039 - 36700160 + 1 = 5242880字节
	return totalSize, actualSize, true
}

func headerInt(s string) int64 {
	if s == "" {
		return -1
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return -1
	}
	return n
}
