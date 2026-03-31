package fsutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyFile copies a single file from src to dst, preserving permissions.
func CopyFile(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	// Handle symlinks
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return fmt.Errorf("readlink %s: %w", src, err)
		}
		return os.Symlink(target, dst)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", src)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("mkdir for %s: %w", dst, err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}

	return nil
}

// CopyDir recursively copies a directory from src to dst, preserving
// symlinks and file permissions.
func CopyDir(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("mkdir %s: %w", dst, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("readdir %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("info %s: %w", srcPath, err)
		}

		// Handle symlinks (entry.Type() preserves symlink info unlike info.Mode())
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(srcPath)
			if err != nil {
				return fmt.Errorf("readlink %s: %w", srcPath, err)
			}
			if err := os.Symlink(target, dstPath); err != nil {
				return fmt.Errorf("symlink %s -> %s: %w", dstPath, target, err)
			}
			continue
		}

		if info.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// CopyDirFiltered copies a directory like CopyDir but only includes
// entries for which the filter function returns true.
func CopyDirFiltered(src, dst string, filter func(name string, isDir bool) bool) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("mkdir %s: %w", dst, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("readdir %s: %w", src, err)
	}

	for _, entry := range entries {
		if !filter(entry.Name(), entry.IsDir()) {
			continue
		}

		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("info %s: %w", srcPath, err)
		}

		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(srcPath)
			if err != nil {
				return fmt.Errorf("readlink %s: %w", srcPath, err)
			}
			if err := os.Symlink(target, dstPath); err != nil {
				return fmt.Errorf("symlink %s -> %s: %w", dstPath, target, err)
			}
			continue
		}

		if info.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
