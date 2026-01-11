package safeguard

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Backup stores a file copy before modification
func Backup(filePath string) error {
	// 1. Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // Nothing to backup
	}

	// 2. Create .ricochet/history directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	historyDir := filepath.Join(homeDir, ".ricochet", "history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("failed to create history dir: %w", err)
	}

	// 3. Create backup filename with timestamp
	filename := filepath.Base(filePath)
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(historyDir, fmt.Sprintf("%s-%s.bak", filename, timestamp))

	// 4. Copy file content
	if err := copyFile(filePath, backupPath); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// Restore restores a file from backup (basic implementation)
func Restore(filePath, backupPath string) error {
	return copyFile(backupPath, filePath)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
