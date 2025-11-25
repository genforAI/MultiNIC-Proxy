package main

import (
	"testing"
)

func TestURLCheck(t *testing.T) {
	// Setup: Clear any existing cache
	GloFileSizeCache.Cache.Range(func(key, value any) bool {
		GloFileSizeCache.Cache.Delete(key)
		return true
	})
	GloCodeCache.Cache.Range(func(key, value any) bool {
		GloCodeCache.Cache.Delete(key)
		return true
	})

	tests := []struct {
		name        string
		url         string
		setup       func()
		expectFound bool
		expectSize  int64
		expectCode  int64
	}{
		{
			name: "URL not in cache with .js extension",
			url:  "https://example.com/script.js",
			setup: func() {
				// No setup needed
			},
			expectFound: false,
			expectSize:  0,
			expectCode:  200,
		},
		{
			name: "URL not in cache with .css extension",
			url:  "https://example.com/style.css",
			setup: func() {
				// No setup needed
			},
			expectFound: false,
			expectSize:  0,
			expectCode:  200,
		},
		{
			name: "URL not in cache with unknown extension",
			url:  "https://example.com/file.zip",
			setup: func() {
				// No setup needed
			},
			expectFound: false,
			expectSize:  -2,
			expectCode:  -2,
		},
		{
			name: "URL in cache",
			url:  "https://example.com/cached_file.zip",
			setup: func() {
				GloFileSizeCache.Cache.Store("https://example.com/cached_file.zip", int64(150000000))
				GloCodeCache.Cache.Store("https://example.com/cached_file.zip", int64(200))
			},
			expectFound: true,
			expectSize:  150000000,
			expectCode:  200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean cache between tests
			GloFileSizeCache.Cache.Range(func(key, value any) bool {
				GloFileSizeCache.Cache.Delete(key)
				return true
			})
			GloCodeCache.Cache.Range(func(key, value any) bool {
				GloCodeCache.Cache.Delete(key)
				return true
			})

			tt.setup()
			found, size, code := URLCheck(tt.url)

			if found != tt.expectFound {
				t.Errorf("expected found=%v, got %v", tt.expectFound, found)
			}
			if size != tt.expectSize {
				t.Errorf("expected size=%d, got %d", tt.expectSize, size)
			}
			if code != tt.expectCode {
				t.Errorf("expected code=%d, got %d", tt.expectCode, code)
			}
		})
	}
}

func TestURLSaveAndCheck(t *testing.T) {
	// Clear cache
	GloFileSizeCache.Cache.Range(func(key, value any) bool {
		GloFileSizeCache.Cache.Delete(key)
		return true
	})
	GloCodeCache.Cache.Range(func(key, value any) bool {
		GloCodeCache.Cache.Delete(key)
		return true
	})

	testURL := "https://example.com/test_file.bin"

	// Initially, URL should not be found
	found, _, _ := URLCheck(testURL)
	if found {
		t.Error("URL should not be found initially")
	}

	// Save URL info
	URLSave(testURL, 200, 50000000)

	// Now URL should be found
	found, size, code := URLCheck(testURL)
	if !found {
		t.Error("URL should be found after save")
	}
	if size != 50000000 {
		t.Errorf("expected size=50000000, got %d", size)
	}
	if code != 200 {
		t.Errorf("expected code=200, got %d", code)
	}
}
