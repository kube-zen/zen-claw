package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		maxBytes  int
		expectLen int // 0 means we don't check exact length
		expectMod bool // true if we expect truncation
	}{
		{
			name:      "no truncation needed",
			input:     "short text",
			maxBytes:  100,
			expectLen: 10,
			expectMod: false,
		},
		{
			name:      "truncation needed",
			input:     string(make([]byte, 10000)), // Much larger than maxBytes
			maxBytes:  1000,
			expectMod: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateOutput(tt.input, tt.maxBytes)
			if !tt.expectMod && result != tt.input {
				t.Errorf("Expected no truncation, got %q", result)
			}
			if tt.expectMod && result == tt.input {
				t.Errorf("Expected truncation but got original")
			}
			if tt.expectLen > 0 && len(result) != tt.expectLen {
				t.Errorf("Expected length %d, got %d", tt.expectLen, len(result))
			}
		})
	}
}

func TestExecTool(t *testing.T) {
	tool := NewExecTool(".")
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
		check   func(result interface{}) error
	}{
		{
			name:    "missing command",
			args:    map[string]interface{}{},
			wantErr: true,
		},
		{
			name: "echo command",
			args: map[string]interface{}{
				"command": "echo hello",
			},
			wantErr: false,
			check: func(result interface{}) error {
				r := result.(map[string]interface{})
				if r["exit_code"] != 0 {
					t.Errorf("Expected exit_code 0, got %v", r["exit_code"])
				}
				return nil
			},
		},
		{
			name: "failing command",
			args: map[string]interface{}{
				"command": "exit 1",
			},
			wantErr: false,
			check: func(result interface{}) error {
				r := result.(map[string]interface{})
				if r["exit_code"] == 0 {
					t.Errorf("Expected non-zero exit code")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil && result != nil {
				tt.check(result)
			}
		})
	}
}

func TestReadFileTool(t *testing.T) {
	// Create temp file for testing
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewReadFileTool(tmpDir)
	ctx := context.Background()

	t.Run("missing path", func(t *testing.T) {
		_, err := tool.Execute(ctx, map[string]interface{}{})
		if err == nil {
			t.Error("Expected error for missing path")
		}
	})

	t.Run("read entire file", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"path": "test.txt",
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		r := result.(map[string]interface{})
		// Check that content is present
		if _, ok := r["content"]; !ok {
			t.Error("Expected content field in result")
		}
	})
}

func TestListDirTool(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte(""), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	tool := NewListDirTool(tmpDir)
	ctx := context.Background()

	t.Run("list current dir", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"path": ".",
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		r := result.(map[string]interface{})
		// Check that files are present
		if _, ok := r["files"]; !ok {
			t.Error("Expected files field in result")
		}
	})
}

func TestWriteFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewWriteFileTool(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
		check   func()
	}{
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
		},
		{
			name: "write new file",
			args: map[string]interface{}{
				"path":    "newfile.txt",
				"content": "hello world",
			},
			wantErr: false,
			check: func() {
				content, err := os.ReadFile(filepath.Join(tmpDir, "newfile.txt"))
				if err != nil {
					t.Errorf("Failed to read written file: %v", err)
				}
				if string(content) != "hello world" {
					t.Errorf("Content mismatch: %q", content)
				}
			},
		},
		{
			name: "overwrite existing",
			args: map[string]interface{}{
				"path":    "newfile.txt",
				"content": "updated content",
			},
			wantErr: false,
			check: func() {
				content, err := os.ReadFile(filepath.Join(tmpDir, "newfile.txt"))
				if err != nil {
					t.Errorf("Failed to read file: %v", err)
				}
				if string(content) != "updated content" {
					t.Errorf("Content mismatch: %q", content)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(ctx, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil {
				tt.check()
			}
		})
	}
}

func TestEditFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "edit.txt")
	os.WriteFile(testFile, []byte("hello world\ngoodbye world"), 0644)

	tool := NewEditFileTool(tmpDir)
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
		check   func()
	}{
		{
			name: "simple replace",
			args: map[string]interface{}{
				"path":       "edit.txt",
				"old_string": "hello",
				"new_string": "hi",
			},
			wantErr: false,
			check: func() {
				content, _ := os.ReadFile(testFile)
				if string(content) != "hi world\ngoodbye world" {
					t.Errorf("Content mismatch: %q", content)
				}
			},
		},
		{
			name: "replace all",
			args: map[string]interface{}{
				"path":        "edit.txt",
				"old_string":  "world",
				"new_string":  "universe",
				"replace_all": true,
			},
			wantErr: false,
			check: func() {
				content, _ := os.ReadFile(testFile)
				if string(content) != "hi universe\ngoodbye universe" {
					t.Errorf("Content mismatch: %q", content)
				}
			},
		},
		{
			name: "string not found",
			args: map[string]interface{}{
				"path":       "edit.txt",
				"old_string": "notfound",
				"new_string": "replacement",
			},
			wantErr: false, // Returns result with error info, not Go error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(ctx, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil {
				tt.check()
			}
		})
	}
}

func TestSystemInfoTool(t *testing.T) {
	tool := NewSystemInfoTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	r := result.(map[string]interface{})

	// Check required fields exist
	requiredFields := []string{"hostname", "current_directory", "time", "pid"}
	for _, field := range requiredFields {
		if _, ok := r[field]; !ok {
			t.Errorf("Missing field: %s", field)
		}
	}
}

func TestAppendFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "append.txt")
	os.WriteFile(testFile, []byte("initial"), 0644)

	tool := NewAppendFileTool(tmpDir)
	ctx := context.Background()

	// Append content
	_, err := tool.Execute(ctx, map[string]interface{}{
		"path":    "append.txt",
		"content": " appended",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	content, _ := os.ReadFile(testFile)
	if string(content) != "initial appended" {
		t.Errorf("Content mismatch: %q", content)
	}
}

func TestSearchFilesTool(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("goodbye world"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file3.go"), []byte("package main"), 0644)

	tool := NewSearchFilesTool(tmpDir)
	ctx := context.Background()

	t.Run("search for pattern", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"pattern": "world",
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		r := result.(map[string]interface{})
		count, ok := r["count"].(int)
		if !ok {
			t.Fatalf("Expected count field as int, got %T", r["count"])
		}
		if count < 2 {
			t.Errorf("Expected at least 2 matches, got %d", count)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"pattern": "nonexistent12345",
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		r := result.(map[string]interface{})
		count, ok := r["count"].(int)
		if !ok {
			t.Fatalf("Expected count field as int, got %T", r["count"])
		}
		if count != 0 {
			t.Errorf("Expected 0 matches, got %d", count)
		}
	})

	t.Run("missing pattern", func(t *testing.T) {
		_, err := tool.Execute(ctx, map[string]interface{}{})
		if err == nil {
			t.Error("Expected error for missing pattern")
		}
	})
}
