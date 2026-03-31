package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	tmp := t.TempDir()

	// Create source file
	src := filepath.Join(tmp, "src.txt")
	if err := os.WriteFile(src, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	// Copy it
	dst := filepath.Join(tmp, "dst.txt")
	if err := CopyFile(src, dst); err != nil {
		t.Fatal(err)
	}

	// Verify contents
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q, want %q", string(data), "hello world")
	}

	// Verify permissions
	srcInfo, _ := os.Stat(src)
	dstInfo, _ := os.Stat(dst)
	if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
		t.Errorf("permissions differ: src=%v, dst=%v", srcInfo.Mode().Perm(), dstInfo.Mode().Perm())
	}
}

func TestCopyFile_Symlink(t *testing.T) {
	tmp := t.TempDir()

	// Create a target file and a symlink to it
	target := filepath.Join(tmp, "target.txt")
	os.WriteFile(target, []byte("target"), 0644)

	link := filepath.Join(tmp, "link.txt")
	os.Symlink(target, link)

	// Copy the symlink
	dst := filepath.Join(tmp, "copied-link.txt")
	if err := CopyFile(link, dst); err != nil {
		t.Fatal(err)
	}

	// Verify it's a symlink pointing to the same target
	linkTarget, err := os.Readlink(dst)
	if err != nil {
		t.Fatal("copied file is not a symlink")
	}
	if linkTarget != target {
		t.Errorf("symlink target = %s, want %s", linkTarget, target)
	}
}

func TestCopyDir(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")

	// Create a directory tree
	os.MkdirAll(filepath.Join(src, "sub", "deep"), 0755)
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(src, "sub", "deep", "c.txt"), []byte("c"), 0644)

	if err := CopyDir(src, dst); err != nil {
		t.Fatal(err)
	}

	// Verify all files exist
	for _, rel := range []string{"a.txt", "sub/b.txt", "sub/deep/c.txt"} {
		path := filepath.Join(dst, rel)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", rel)
		}
	}

	// Verify content
	data, _ := os.ReadFile(filepath.Join(dst, "sub", "deep", "c.txt"))
	if string(data) != "c" {
		t.Errorf("got %q, want %q", string(data), "c")
	}
}

func TestCopyDir_PreservesSymlinks(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")

	os.MkdirAll(src, 0755)

	// Create a file and symlink inside src
	os.WriteFile(filepath.Join(src, "real.txt"), []byte("real"), 0644)
	os.Symlink("real.txt", filepath.Join(src, "link.txt"))

	if err := CopyDir(src, dst); err != nil {
		t.Fatal(err)
	}

	// Verify symlink is preserved
	target, err := os.Readlink(filepath.Join(dst, "link.txt"))
	if err != nil {
		t.Fatal("expected symlink, got regular file")
	}
	if target != "real.txt" {
		t.Errorf("symlink target = %s, want real.txt", target)
	}
}

func TestCopyDirFiltered(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	dst := filepath.Join(tmp, "dst")

	os.MkdirAll(src, 0755)
	os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0644)
	os.WriteFile(filepath.Join(src, "skip.log"), []byte("skip"), 0644)

	filter := func(name string, isDir bool) bool {
		return filepath.Ext(name) != ".log"
	}

	if err := CopyDirFiltered(src, dst, filter); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dst, "keep.txt")); os.IsNotExist(err) {
		t.Error("keep.txt should exist")
	}
	if _, err := os.Stat(filepath.Join(dst, "skip.log")); !os.IsNotExist(err) {
		t.Error("skip.log should not exist")
	}
}

func TestAtomicSymlink(t *testing.T) {
	tmp := t.TempDir()

	targetA := filepath.Join(tmp, "a")
	targetB := filepath.Join(tmp, "b")
	os.MkdirAll(targetA, 0755)
	os.MkdirAll(targetB, 0755)

	link := filepath.Join(tmp, "link")

	// Create initial symlink
	if err := os.Symlink(targetA, link); err != nil {
		t.Fatal(err)
	}

	got, _ := os.Readlink(link)
	if got != targetA {
		t.Fatalf("initial symlink = %q, want %q", got, targetA)
	}

	// Atomically replace
	if err := AtomicSymlink(targetB, link); err != nil {
		t.Fatal(err)
	}

	got, _ = os.Readlink(link)
	if got != targetB {
		t.Errorf("after atomic swap: symlink = %q, want %q", got, targetB)
	}

	// No temp files should remain
	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if e.Name() == "link.tmp" {
			t.Error("temp symlink should be cleaned up")
		}
	}
}
