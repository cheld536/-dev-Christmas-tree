package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallJournalRollback(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.exe")
	newFile := filepath.Join(dir, "new.scr")
	if err := os.WriteFile(existing, []byte("기존 파일"), 0600); err != nil {
		t.Fatal(err)
	}
	var journal installJournal
	if err := journal.write(existing, []byte("새 파일")); err != nil {
		t.Fatal(err)
	}
	if err := journal.write(newFile, []byte("새 파일")); err != nil {
		t.Fatal(err)
	}
	if err := journal.rollback(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(existing)
	if err != nil || string(data) != "기존 파일" {
		t.Fatalf("기존 파일 복원 실패. %q, %v", data, err)
	}
	if _, err := os.Stat(newFile); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("새 파일 제거 실패. %v", err)
	}
}
