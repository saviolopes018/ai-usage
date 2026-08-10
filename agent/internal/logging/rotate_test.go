package logging

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotatesAndLimitsBackups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.log")
	w, err := Open(path, 10, 2)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := w.Write([]byte("123456\n")); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatal("first backup missing")
	}
	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Fatal("too many backups")
	}
}
