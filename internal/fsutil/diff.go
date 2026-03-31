package fsutil

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
)

// DiffResult holds the comparison between two directories.
type DiffResult struct {
	OnlyInA   []string // Files only in directory A
	OnlyInB   []string // Files only in directory B
	Different []string // Files present in both but with different contents
	Identical []string // Files present in both with identical contents
}

// DiffDirs compares two directory trees and returns the differences.
// Paths are reported relative to the roots.
func DiffDirs(dirA, dirB string) (*DiffResult, error) {
	filesA, err := listFilesRelative(dirA)
	if err != nil {
		return nil, err
	}
	filesB, err := listFilesRelative(dirB)
	if err != nil {
		return nil, err
	}

	setA := make(map[string]bool, len(filesA))
	for _, f := range filesA {
		setA[f] = true
	}
	setB := make(map[string]bool, len(filesB))
	for _, f := range filesB {
		setB[f] = true
	}

	result := &DiffResult{}

	for _, f := range filesA {
		if !setB[f] {
			result.OnlyInA = append(result.OnlyInA, f)
		}
	}

	for _, f := range filesB {
		if !setA[f] {
			result.OnlyInB = append(result.OnlyInB, f)
		}
	}

	for _, f := range filesA {
		if !setB[f] {
			continue
		}
		pathA := filepath.Join(dirA, f)
		pathB := filepath.Join(dirB, f)

		dataA, errA := os.ReadFile(pathA)
		dataB, errB := os.ReadFile(pathB)

		if errA != nil || errB != nil {
			result.Different = append(result.Different, f)
			continue
		}

		if bytes.Equal(dataA, dataB) {
			result.Identical = append(result.Identical, f)
		} else {
			result.Different = append(result.Different, f)
		}
	}

	return result, nil
}

func listFilesRelative(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		files = append(files, rel)
		return nil
	})
	sort.Strings(files)
	return files, err
}
