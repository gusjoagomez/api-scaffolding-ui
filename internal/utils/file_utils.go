package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

func BackupFileIfExists(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // Archivo no existe, no hay necesidad de backup
	}

	ext := filepath.Ext(filePath)
	base := strings.TrimSuffix(filePath, ext)
	backupPath := fmt.Sprintf("%s.%s.bak", base, time.Now().Format("20060102"))

	return os.Rename(filePath, backupPath)
}

func EnsureDirectory(dirPath string) error {
	return os.MkdirAll(dirPath, 0755)
}

func WriteFileWithBackup(filePath string, content []byte) error {
	// Asegurar que el directorio existe
	if err := EnsureDirectory(filepath.Dir(filePath)); err != nil {
		return fmt.Errorf("error creating directory: %v", err)
	}

	// Hacer backup si el archivo existe
	if err := BackupFileIfExists(filePath); err != nil {
		return fmt.Errorf("error backing up file: %v", err)
	}

	// Escribir archivo
	return os.WriteFile(filePath, content, 0644)
}

func TitleFirst(s string) string {
	if s == "" {
		return s
	}

	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
