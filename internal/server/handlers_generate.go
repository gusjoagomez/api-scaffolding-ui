package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"api-scaffolding/internal/database"
	"api-scaffolding/internal/generator"
	"api-scaffolding/internal/models"
)

// handleFileTemplatesList returns the list of file_templates as JSON (used by the front-end).
func (s *Server) handleFileTemplatesList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT id, version, grouptype, category, name, path, file, template, source, orderlist, visible, typefile
		FROM %s.file_templates
		WHERE visible = 1
		ORDER BY orderlist`, s.cfg.DBSchema))
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// DTO with plain strings — avoids sql.NullString serialising as {"String":"...","Valid":true}
	type FileTemplateDTO struct {
		ID        int    `json:"id"`
		Version   string `json:"version"`
		GroupType string `json:"grouptype"`
		Category  string `json:"category"`
		Name      string `json:"name"`
		Path      string `json:"path"`
		File      string `json:"file"`
		Template  string `json:"template"`
		Source    string `json:"source"`
		OrderList int32  `json:"orderlist"`
		Visible   int16  `json:"visible"`
		TypeFile  string `json:"typefile"`
	}

	var result []FileTemplateDTO
	for rows.Next() {
		var (
			version, grouptype, category, name sql.NullString
			path, file, tmpl, source           sql.NullString
			orderlist                          sql.NullInt32
			visible                            sql.NullInt16
			typefile                           sql.NullString
			id                                 int
		)
		if err := rows.Scan(
			&id, &version, &grouptype, &category, &name,
			&path, &file, &tmpl, &source,
			&orderlist, &visible, &typefile,
		); err != nil {
			renderError(w, err, http.StatusInternalServerError)
			return
		}
		result = append(result, FileTemplateDTO{
			ID:        id,
			Version:   version.String,
			GroupType: grouptype.String,
			Category:  category.String,
			Name:      name.String,
			Path:      path.String,
			File:      file.String,
			Template:  tmpl.String,
			Source:    source.String,
			OrderList: orderlist.Int32,
			Visible:   visible.Int16,
			TypeFile:  typefile.String,
		})
	}

	if result == nil {
		result = []FileTemplateDTO{} // return [] not null
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleGenerate processes the "Generate" button: receives selected tables + selected
// file_templates, resolves paths, applies Go templates and writes yaml files.
func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectName := r.FormValue("projectname")
	connName := r.FormValue("connection")
	selectedTables := r.Form["tables[]"]
	selectedTemplateIDs := r.Form["file_templates[]"]

	if projectName == "" || connName == "" {
		http.Error(w, "projectname and connection are required", http.StatusBadRequest)
		return
	}
	if len(selectedTables) == 0 || len(selectedTemplateIDs) == 0 {
		http.Error(w, "select at least one table and one template", http.StatusBadRequest)
		return
	}

	// --- 1. Get project rootdir ---
	var rootDir sql.NullString
	if err := s.db.QueryRow(fmt.Sprintf(`SELECT rootdir FROM %s.project WHERE projectname=$1`, s.cfg.DBSchema), projectName).Scan(&rootDir); err != nil {
		renderError(w, fmt.Errorf("project not found: %v", err), http.StatusInternalServerError)
		return
	}

	// --- 2. Get selected file_templates ---
	placeholders := make([]string, len(selectedTemplateIDs))
	args := make([]interface{}, len(selectedTemplateIDs))
	for i, id := range selectedTemplateIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	query := fmt.Sprintf(`
		SELECT id, version, grouptype, category, name, path, file, template, source, orderlist, visible, typefile
		FROM %s.file_templates
		WHERE id IN (%s)
		ORDER BY orderlist`, s.cfg.DBSchema, strings.Join(placeholders, ","))

	tplRows, err := s.db.Query(query, args...)
	if err != nil {
		renderError(w, err, http.StatusInternalServerError)
		return
	}
	defer tplRows.Close()

	var fileTemplates []models.FileTemplate
	for tplRows.Next() {
		var ft models.FileTemplate
		if err := tplRows.Scan(
			&ft.ID, &ft.Version, &ft.GroupType, &ft.Category, &ft.Name,
			&ft.Path, &ft.File, &ft.Template, &ft.Source,
			&ft.OrderList, &ft.Visible, &ft.TypeFile,
		); err != nil {
			renderError(w, err, http.StatusInternalServerError)
			return
		}
		fileTemplates = append(fileTemplates, ft)
	}

	// --- 3. Get connection details to connect to target DB ---
	var dbType, dbHost, dbPort, dbUser, dbPass, dbNameNS, dbSchemaNS, dbSslMode, dbTimezone sql.NullString
	row := s.db.QueryRow(fmt.Sprintf(`
		SELECT dbtype, dbhost, dbport, dbuser, dbpass, dbname, dbschema, dbsslmode, dbtimezone
		FROM %s.dbconn
		WHERE projectname=$1 AND connection=$2`, s.cfg.DBSchema), projectName, connName)
	if err := row.Scan(&dbType, &dbHost, &dbPort, &dbUser, &dbPass, &dbNameNS, &dbSchemaNS, &dbSslMode, &dbTimezone); err != nil {
		renderError(w, fmt.Errorf("connection not found: %v", err), http.StatusInternalServerError)
		return
	}

	// --- 4. Connect to target database and get table metadata ---
	cfg := &database.DatabaseConfig{
		Driver:   dbType.String,
		Host:     dbHost.String,
		Port:     dbPort.String,
		Username: dbUser.String,
		Password: dbPass.String,
		Database: dbNameNS.String,
		SSLMode:  dbSslMode.String,
		Timezone: dbTimezone.String,
	}
	scanner := database.NewScanner()
	if err := scanner.Connect(cfg); err != nil {
		renderError(w, fmt.Errorf("cannot connect to target db: %v", err), http.StatusInternalServerError)
		return
	}
	defer scanner.Disconnect()

	targetSchema := dbSchemaNS.String
	if targetSchema == "" {
		targetSchema = "public"
	}

	allTables, err := scanner.GetTables(targetSchema, []string{"*"})
	if err != nil {
		renderError(w, fmt.Errorf("cannot get tables: %v", err), http.StatusInternalServerError)
		return
	}

	// Build a lookup by table name
	tableMap := make(map[string]database.Table)
	for _, t := range allTables {
		tableMap[strings.ToLower(t.Name)] = t
	}

	// --- 5. Build the TemplateProcessor loading from templatesgen/ ---
	tp, err := generator.NewTemplateProcessor("templatesgen")
	if err != nil {
		renderError(w, fmt.Errorf("cannot load templates: %v", err), http.StatusInternalServerError)
		return
	}

	// Generator config (needed for prepareTemplateData)
	genConfig := &generator.Config{
		DBDriver:         dbType.String,
		DBHost:           dbHost.String,
		DBPort:           dbPort.String,
		DBUsername:       dbUser.String,
		DBPassword:       dbPass.String,
		DBName:           dbNameNS.String,
		DBSSLMode:        dbSslMode.String,
		ProjectDir:       rootDir.String,
		ProjectSchema:    targetSchema,
		ProjectFileTypes: "yaml",
		ProjectRelations: []string{"*"},
	}
	gen := generator.NewGenerator(genConfig, scanner, tp)

	type GenerateResult struct {
		File    string `json:"file"`
		Status  string `json:"status"`
		Message string `json:"message,omitempty"`
	}
	var results []GenerateResult

	// --- 6. For each selected table × each selected file_template → generate ---
	for _, tableName := range selectedTables {
		table, ok := tableMap[strings.ToLower(tableName)]
		if !ok {
			results = append(results, GenerateResult{
				File:    tableName,
				Status:  "error",
				Message: "table not found in database",
			})
			continue
		}

		templateData := gen.PrepareTemplateDataPublic(table, allTables)

		// entity name (singular, lowercase) for path substitutions
		entityName := strings.ToLower(templateData["EntityName"].(string))

		for _, ft := range fileTemplates {
			templateFile := strings.TrimSpace(ft.Template.String)
			// template file_templates stores paths like "templatesgen/entidad_list.tpl"
			// TemplateProcessor keys are the basename
			templateBasename := filepath.Base(templateFile)

			// Resolve output path: replace [rootprj] and [entity]
			outPath := strings.ReplaceAll(ft.Path.String, "[rootprj]", rootDir.String)
			outPath = strings.ReplaceAll(outPath, "[entity]", strings.ToLower(tableName))
			outFile := strings.ReplaceAll(ft.File.String, "[entity]", entityName)
			fullPath := filepath.Join(outPath, outFile)

			content, err := tp.Process(templateBasename, templateData)
			if err != nil {
				results = append(results, GenerateResult{
					File:    fullPath,
					Status:  "error",
					Message: fmt.Sprintf("template error: %v", err),
				})
				continue
			}

			if err := writeFileSafe(fullPath, content); err != nil {
				results = append(results, GenerateResult{
					File:    fullPath,
					Status:  "error",
					Message: fmt.Sprintf("write error: %v", err),
				})
				continue
			}

			results = append(results, GenerateResult{
				File:   fullPath,
				Status: "ok",
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
	})
}

// writeFileSafe creates directories, backs-up existing file and writes content.
func writeFileSafe(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		backupPath := fmt.Sprintf("%s.%s.bak", path, time.Now().Format("20060102150405"))
		_ = os.Rename(path, backupPath)
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// processTemplate executes a Go text/template string with the given data.
// Used internally for the generate handler.
func processTemplate(templateStr string, data interface{}) (string, error) {
	tmpl, err := template.New("inline").Parse(templateStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
