package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopShortcutCanBeRolledBack(t *testing.T) {
	dir, err := desktopDirectory()
	if err != nil || !filepath.IsAbs(dir) {
		t.Fatalf("Windows 바탕 화면 경로 오류. %q, %v", dir, err)
	}
	app, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "크리스마스 트리.lnk")
	var journal installJournal
	if err := journal.desktopShortcut(path, app); err != nil {
		t.Fatal(err)
	}
	if err := journal.desktopShortcut(path, app); err != nil {
		target, readErr := shortcutTarget(path)
		t.Fatalf("같은 바로가기 다시 사용 실패. 대상 %q, 기댓값 %q, 조회 오류 %v, 설치 오류 %v", target, app, readErr, err)
	}
	if err := journal.desktopShortcut(path, filepath.Join(filepath.Dir(app), "other.exe")); err == nil {
		t.Fatal("다른 대상을 가리키는 바로가기를 보호해야 합니다.")
	}
	if target, err := shortcutTarget(path); err != nil || !sameTarget(target, app) {
		t.Fatalf("바로가기 대상 확인 실패. %q, %v", target, err)
	}
	if err := journal.rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("생성한 바로가기 제거 실패. %v", err)
	}
}

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
