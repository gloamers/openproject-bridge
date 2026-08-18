package knowledge_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gloamers/openproject-bridge/internal/knowledge"
)

func TestWriteADR(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	svc := &knowledge.Service{ADRDir: filepath.Join(dir, "docs", "adr")}
	path, err := svc.WriteADR(12, "Use SQLite for mappings", "https://github.com/a/b/issues/12", "http://op/wp/1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "Use SQLite for mappings") {
		t.Fatalf("content=%s", b)
	}
	if !strings.Contains(path, "0012-") {
		t.Fatalf("path=%s", path)
	}
}
