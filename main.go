package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"api-scaffolding/internal/config"
	"api-scaffolding/internal/database"
	"api-scaffolding/internal/generator"
)

func main() {
	// Parsear argumentos de línea de comandos
	var configPath string
	flag.StringVar(&configPath, "configdb", "", "Path to .envapi configuration file")
	flag.Parse()

	// Si no se especifica, usar valor por defecto
	if configPath == "" {
		configPath = "config/.envapi"
	}

	// Cargar configuración
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	fmt.Printf("Configuration loaded from: %s\n", configPath)
	fmt.Printf("Database: %s://%s:%s/%s\n", cfg.DBDriver, cfg.DBHost, cfg.DBPort, cfg.DBName)
	fmt.Printf("Output directory: %s\n", cfg.ProjectDir)

	// Crear scanner de base de datos
	dbScanner := database.NewScanner()

	// Configurar conexión a base de datos
	dbConfig := &database.DatabaseConfig{
		Driver:   cfg.DBDriver,
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		Username: cfg.DBUsername,
		Password: cfg.DBPassword,
		Database: cfg.DBName,
		SSLMode:  cfg.DBSSLMode,
		Timezone: cfg.DBTimezone,
	}

	// Conectar a la base de datos
	if err := dbScanner.Connect(dbConfig); err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer dbScanner.Disconnect()

	fmt.Println("Connected to database successfully")

	// Crear directorio de templates si no existe
	templatesDir := "templates"
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		log.Fatalf("Error creating templates directory: %v", err)
	}

	// Crear processor de templates
	templateProcessor, err := generator.NewTemplateProcessor(templatesDir)
	if err != nil {
		log.Fatalf("Error creating template processor: %v", err)
	}

	// Crear generador
	genConfig := &generator.Config{
		DBDriver:         cfg.DBDriver,
		DBHost:           cfg.DBHost,
		DBPort:           cfg.DBPort,
		DBUsername:       cfg.DBUsername,
		DBPassword:       cfg.DBPassword,
		DBName:           cfg.DBName,
		DBSSLMode:        cfg.DBSSLMode,
		DBTimezone:       cfg.DBTimezone,
		ProjectFileTypes: cfg.ProjectFileTypes,
		ProjectDir:       cfg.ProjectDir,
		ProjectSchema:    cfg.ProjectSchema,
		ProjectTables:    cfg.ProjectTables,
		ProjectRelations: cfg.ProjectRelations,
	}

	gen := generator.NewGenerator(genConfig, dbScanner, templateProcessor)

	// Generar archivos
	if err := gen.Generate(); err != nil {
		log.Fatalf("Error during generation: %v", err)
	}

	fmt.Println("Scaffolding completed successfully!")
}

func init() {
	// Configurar log
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Crear directorio de configuración si no existe
	if _, err := os.Stat("config"); os.IsNotExist(err) {
		if err := os.MkdirAll("config", 0755); err != nil {
			log.Printf("Warning: Could not create config directory: %v", err)
		}
	}

	// Crear archivo de configuración de ejemplo si no existe
	configExample := `# ===================
# CONFIGURACIÓN DE BASE DE DATOS
DB_DRIVER=postgres
DB_HOST=localhost
DB_PORT=5432
DB_USERNAME=postgres
DB_PASSWORD=password
DB_NAME=proyecto1
DB_SSL_MODE=disable
DB_TIMEZONE=UTC
#
# UBICACION A GENERAR LOS ARCHIVOS
#
PROJECT_FILE_TYPES=yaml
PROJECT_DIR=./apis/
PROJECT_SCHEMA=public
PROJECT_TABLES=*
PROJECT_RELATIONS=*
# ===================`

	configFile := "config/.envapi.example"
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if err := os.WriteFile(configFile, []byte(configExample), 0644); err != nil {
			log.Printf("Warning: Could not create example config file: %v", err)
		}
	}
}
