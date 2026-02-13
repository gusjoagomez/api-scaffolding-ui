package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DBDriver         string
	DBHost           string
	DBPort           string
	DBUsername       string
	DBPassword       string
	DBName           string
	DBSSLMode        string
	DBTimezone       string
	ProjectFileTypes string
	ProjectDir       string
	ProjectSchema    string
	ProjectTables    []string
	ProjectRelations []string
}

func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = "config/.envapi"
	}

	// Intentar cargar el archivo de configuración
	if err := godotenv.Load(configPath); err != nil {
		return nil, fmt.Errorf("error loading config file %s: %v", configPath, err)
	}

	config := &Config{
		DBDriver:         getEnv("DB_DRIVER", "postgres"),
		DBHost:           getEnv("DB_HOST", "localhost"),
		DBPort:           getEnv("DB_PORT", "5432"),
		DBUsername:       getEnv("DB_USERNAME", ""),
		DBPassword:       getEnv("DB_PASSWORD", ""),
		DBName:           getEnv("DB_NAME", ""),
		DBSSLMode:        getEnv("DB_SSL_MODE", "disable"),
		DBTimezone:       getEnv("DB_TIMEZONE", "UTC"),
		ProjectFileTypes: strings.ToLower(getEnv("PROJECT_FILE_TYPES", "yaml")),
		ProjectDir:       getEnv("PROJECT_DIR", "./apis/"),
		ProjectSchema:    getEnv("PROJECT_SCHEMA", "public"),
	}

	// Parsear tablas
	tablesStr := getEnv("PROJECT_TABLES", "*")
	if tablesStr == "*" {
		config.ProjectTables = []string{"*"}
	} else {
		config.ProjectTables = splitAndTrim(tablesStr, ",")
	}

	// Parsear relaciones
	relationsStr := getEnv("PROJECT_RELATIONS", "")
	if relationsStr == "*" {
		config.ProjectRelations = []string{"*"}
	} else {
		config.ProjectRelations = splitAndTrim(relationsStr, ",")
	}

	// Asegurar que el directorio termina con /
	if !strings.HasSuffix(config.ProjectDir, string(filepath.Separator)) {
		config.ProjectDir += string(filepath.Separator)
	}

	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func splitAndTrim(str, sep string) []string {
	if str == "" {
		return []string{}
	}
	parts := strings.Split(str, sep)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
