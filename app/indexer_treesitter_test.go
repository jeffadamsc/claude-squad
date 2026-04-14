package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTreeSitterIndexerBuild(t *testing.T) {
	tmp := t.TempDir()

	// Create minimal git repo
	os.MkdirAll(filepath.Join(tmp, ".git"), 0755)

	// Create a Go file
	goCode := []byte(`package main

func Hello() string {
    return "hello"
}
`)
	os.WriteFile(filepath.Join(tmp, "main.go"), goCode, 0644)

	// Initialize git and add file
	exec.Command("git", "-C", tmp, "init").Run()
	exec.Command("git", "-C", tmp, "add", "main.go").Run()

	idx := NewTreeSitterIndexer(tmp)
	idx.Start()

	// Wait for build
	time.Sleep(100 * time.Millisecond)

	// Check files
	files := idx.Files()
	if len(files) != 1 || files[0] != "main.go" {
		t.Errorf("Files() = %v, want [main.go]", files)
	}

	// Check symbols
	defs := idx.Lookup("Hello")
	if len(defs) != 1 {
		t.Errorf("Lookup(Hello) = %d results, want 1", len(defs))
	}

	idx.Stop()
}

func TestTreeSitterIndexerNew(t *testing.T) {
	tmp := t.TempDir()

	// Create a minimal git repo
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	idx := NewTreeSitterIndexer(tmp)
	if idx == nil {
		t.Fatal("expected non-nil indexer")
	}
	if idx.Worktree() != tmp {
		t.Errorf("worktree = %q, want %q", idx.Worktree(), tmp)
	}
}

func TestGetSymbolContent(t *testing.T) {
	tmp := t.TempDir()

	// Create minimal git repo
	os.MkdirAll(filepath.Join(tmp, ".git"), 0755)
	exec.Command("git", "-C", tmp, "init").Run()

	// Create a Go file with a known function
	goCode := []byte(`package main

func Hello() string {
	return "hello"
}

func Goodbye() string {
	return "goodbye"
}
`)
	os.WriteFile(filepath.Join(tmp, "main.go"), goCode, 0644)
	exec.Command("git", "-C", tmp, "add", "main.go").Run()

	idx := NewTreeSitterIndexer(tmp)
	idx.Start()
	time.Sleep(200 * time.Millisecond)

	// Look up Hello function
	syms := idx.LookupSymbol("Hello")
	if len(syms) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(syms))
	}

	sym := syms[0]

	// Verify byte offsets are set
	if sym.StartByte == 0 && sym.EndByte == 0 {
		t.Error("byte offsets not set")
	}

	// Get content using byte offsets
	content, err := idx.GetSymbolContent(sym)
	if err != nil {
		t.Fatalf("GetSymbolContent failed: %v", err)
	}

	// Should contain the function definition
	if !strings.Contains(content, "func Hello()") {
		t.Errorf("content should contain 'func Hello()', got: %s", content)
	}
	if !strings.Contains(content, "return \"hello\"") {
		t.Errorf("content should contain return statement, got: %s", content)
	}

	// Should NOT contain Goodbye function
	if strings.Contains(content, "Goodbye") {
		t.Errorf("content should not contain Goodbye function, got: %s", content)
	}

	idx.Stop()
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"a", 1},
		{"test", 1},
		{"hello world", 3}, // 11 chars / 4 = 2.75 -> 3
		{"func Hello() string { return \"hello\" }", 10}, // 38 chars / 4 = 9.5 -> 10
	}

	for _, tt := range tests {
		got := EstimateTokens(tt.input)
		if got != tt.expected {
			t.Errorf("EstimateTokens(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}
