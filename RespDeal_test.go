package main

import (
	"net/http"
	"testing"
)

// TestRespDeal tests the RespDeal function which processes HTTP response headers
func TestRespDeal_StatusOK(t *testing.T) {
	tests := []struct {
		name           string
		header         http.Header
		statusCode     int
		expectChunks   bool
		expectFileSize int64
		expectError    bool
	}{
		{
			name: "200 OK with Content-Length less than ExceedSize",
			header: http.Header{
				"Content-Length": []string{"1000000"},
				"Accept-Ranges":  []string{"bytes"},
			},
			statusCode:     http.StatusOK,
			expectChunks:   false,
			expectFileSize: 1000000,
			expectError:    false,
		},
		{
			name: "200 OK with Content-Length greater than ExceedSize",
			header: http.Header{
				"Content-Length": []string{"200000000"}, // > 100MB
				"Accept-Ranges":  []string{"bytes"},
			},
			statusCode:     http.StatusOK,
			expectChunks:   true,
			expectFileSize: 200000000,
			expectError:    false,
		},
		{
			name: "200 OK without Accept-Ranges",
			header: http.Header{
				"Content-Length": []string{"200000000"},
			},
			statusCode:     http.StatusOK,
			expectChunks:   false,
			expectFileSize: 200000000,
			expectError:    false,
		},
		{
			name:           "200 OK without Content-Length",
			header:         http.Header{},
			statusCode:     http.StatusOK,
			expectChunks:   false,
			expectFileSize: -1,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ifChunks, fileSize, _, err := RespDeal(tt.header, tt.statusCode)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if ifChunks != tt.expectChunks {
				t.Errorf("expected ifChunks=%v, got %v", tt.expectChunks, ifChunks)
			}
			if fileSize != tt.expectFileSize {
				t.Errorf("expected fileSize=%d, got %d", tt.expectFileSize, fileSize)
			}
		})
	}
}

func TestRespDeal_StatusPartialContent(t *testing.T) {
	tests := []struct {
		name           string
		header         http.Header
		statusCode     int
		expectChunks   bool
		expectFileSize int64
		expectError    bool
	}{
		{
			name: "206 with valid Content-Range",
			header: http.Header{
				"Content-Range": []string{"bytes 0-999/10000"},
			},
			statusCode:     http.StatusPartialContent,
			expectChunks:   false,
			expectFileSize: 10000,
			expectError:    false,
		},
		{
			name: "206 without Content-Range",
			header: http.Header{},
			statusCode:     http.StatusPartialContent,
			expectChunks:   false,
			expectFileSize: -1,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ifChunks, fileSize, _, err := RespDeal(tt.header, tt.statusCode)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if ifChunks != tt.expectChunks {
				t.Errorf("expected ifChunks=%v, got %v", tt.expectChunks, ifChunks)
			}
			if fileSize != tt.expectFileSize {
				t.Errorf("expected fileSize=%d, got %d", tt.expectFileSize, fileSize)
			}
		})
	}
}

// TestParseContentRangeTotal tests the parseContentRangeTotal function
func TestParseContentRangeTotal(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectPartSz  int64
		expectTotalSz int64
		expectOk      bool
	}{
		{
			name:          "Valid range with total",
			input:         "bytes 0-999/10000",
			expectPartSz:  10000,
			expectTotalSz: 1000,
			expectOk:      true,
		},
		{
			name:          "Valid range with unknown total",
			input:         "bytes 0-999/*",
			expectPartSz:  -1,
			expectTotalSz: 1000,
			expectOk:      true, // parseContentRangeTotal returns true as long as part size is valid
		},
		{
			name:          "416 style range",
			input:         "bytes */10000",
			expectPartSz:  -1,
			expectTotalSz: 10000,
			expectOk:      false, // This format doesn't have valid part size
		},
		{
			name:          "Empty string",
			input:         "",
			expectPartSz:  -1,
			expectTotalSz: -1,
			expectOk:      false,
		},
		{
			name:          "Large file range",
			input:         "bytes 36700160-41943039/207322416",
			expectPartSz:  207322416,
			expectTotalSz: 5242880,
			expectOk:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			partSz, totalSz, ok := parseContentRangeTotal(tt.input)
			if ok != tt.expectOk {
				t.Errorf("expected ok=%v, got %v", tt.expectOk, ok)
			}
			if partSz != tt.expectPartSz {
				t.Errorf("expected partSz=%d, got %d", tt.expectPartSz, partSz)
			}
			if totalSz != tt.expectTotalSz {
				t.Errorf("expected totalSz=%d, got %d", tt.expectTotalSz, totalSz)
			}
		})
	}
}

// TestHeaderInt tests the headerInt function
func TestHeaderInt(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect int64
	}{
		{"Valid positive number", "12345", 12345},
		{"Zero", "0", 0},
		{"Empty string", "", -1},
		{"Negative number", "-100", -1},
		{"Invalid string", "abc", -1},
		{"Large number", "9223372036854775807", 9223372036854775807},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := headerInt(tt.input)
			if result != tt.expect {
				t.Errorf("expected %d, got %d", tt.expect, result)
			}
		})
	}
}
