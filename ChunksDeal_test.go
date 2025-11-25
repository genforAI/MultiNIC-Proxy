package main

import (
	"testing"
)

// TestChunksDirectTaskGet tests the ChunksDirectTaskGet function
func TestChunksDirectTaskGet(t *testing.T) {
	tests := []struct {
		name           string
		allChunks      []ChunkTask
		sizeChunksDD   int64
		expectChunks   int
		expectNextIdx  int
		expectError    bool
	}{
		{
			name: "Basic chunking",
			allChunks: []ChunkTask{
				{Index: 0, Start: 0, End: 999999},
				{Index: 1, Start: 1000000, End: 1999999},
				{Index: 2, Start: 2000000, End: 2999999},
			},
			sizeChunksDD:  500000,
			expectChunks:  1,
			expectNextIdx: 1,
			expectError:   false,
		},
		{
			name: "Multiple chunks under size",
			allChunks: []ChunkTask{
				{Index: 0, Start: 0, End: 99999},
				{Index: 1, Start: 100000, End: 199999},
				{Index: 2, Start: 200000, End: 299999},
			},
			sizeChunksDD:  500000,
			expectChunks:  3,
			expectNextIdx: 3,
			expectError:   true, // All chunks consumed but still under size
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks, nextIdx, err := ChunksDirectTaskGet(tt.allChunks, tt.sizeChunksDD)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if len(chunks) != tt.expectChunks {
				t.Errorf("expected %d chunks, got %d", tt.expectChunks, len(chunks))
			}
			if nextIdx != tt.expectNextIdx {
				t.Errorf("expected nextIdx=%d, got %d", tt.expectNextIdx, nextIdx)
			}
		})
	}
}

// TestTaskRetryable tests the taskRetryable function
func TestTaskRetryable(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		expect bool
	}{
		{
			name:   "Retryable error",
			err:    &mockError{msg: "retryable: connection reset"},
			expect: true,
		},
		{
			name:   "Non-retryable error",
			err:    &mockError{msg: "fatal error"},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := taskRetryable(tt.err)
			if result != tt.expect {
				t.Errorf("expected %v, got %v", tt.expect, result)
			}
		})
	}
}

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}
