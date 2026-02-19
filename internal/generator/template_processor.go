package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// Definir tipos necesarios
type Column struct {
	Name         string
	DataType     string
	IsNullable   bool
	IsPrimaryKey bool
	IsForeignKey bool
	DefaultValue *string
	MaxLength    *int
	Comment      string
}

type ForeignKey struct {
	ColumnName       string
	ReferencedTable  string
	ReferencedColumn string
	ConstraintName   string
}

type TemplateProcessor struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

func NewTemplateProcessor(templatesDir string) (*TemplateProcessor, error) {
	tp := &TemplateProcessor{
		templates: make(map[string]*template.Template),
		funcMap: template.FuncMap{
			"toSnakeCase":           toSnakeCase,
			"toCamelCase":           toCamelCase,
			"toPascalCase":          toPascalCase,
			"toLowerCase":           strings.ToLower,
			"toUpperCase":           strings.ToUpper,
			"pluralize":             pluralize,
			"singularize":           singularize,
			"formatType":            formatType,
			"getValidation":         getValidation,
			"getDefault":            getDefault,
			"getDefaultValue":       getDefaultValue,
			"now":                   time.Now,
			"contains":              strings.Contains,
			"hasPrefix":             strings.HasPrefix,
			"hasSuffix":             strings.HasSuffix,
			"replace":               strings.ReplaceAll,
			"trim":                  strings.TrimSpace,
			"join":                  strings.Join,
			"split":                 strings.Split,
			"quote":                 strconv.Quote,
			"toString":              toString,
			"hasField":              hasField,
			"isAuditField":          isAuditField,
			"shouldIncludeInUpdate": shouldIncludeInUpdate,
			"coalesceFunc":          coalesceFunc,
			"nowFunc":               nowFunc,
			"list":                  list,
			"append":                appendFunc,
			"gt":                    greaterThan,
			"indent":                indentFunc,
			// Sprig-compatible helpers used in templates
			"default": defaultValue,
			"empty":   isEmpty,
			"add":     addInt,
		},
	}

	// Cargar templates
	if err := tp.loadTemplates(templatesDir); err != nil {
		return nil, err
	}

	return tp, nil
}

func (tp *TemplateProcessor) loadTemplates(templatesDir string) error {
	// Crear directorio si no existe
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return fmt.Errorf("error creating templates directory: %v", err)
	}

	templateFiles, err := filepath.Glob(filepath.Join(templatesDir, "*.tpl"))
	if err != nil {
		return fmt.Errorf("error listing template files: %v", err)
	}

	// Si no hay templates, crear algunos por defecto
	if len(templateFiles) == 0 {
		if err := tp.createDefaultTemplates(templatesDir); err != nil {
			return fmt.Errorf("error creating default templates: %v", err)
		}
		// Volver a listar
		templateFiles, err = filepath.Glob(filepath.Join(templatesDir, "*.tpl"))
		if err != nil {
			return fmt.Errorf("error listing template files: %v", err)
		}
	}

	for _, templateFile := range templateFiles {
		templateName := filepath.Base(templateFile)
		content, err := os.ReadFile(templateFile)
		if err != nil {
			return fmt.Errorf("error reading template %s: %v", templateFile, err)
		}

		tmpl, err := template.New(templateName).Funcs(tp.funcMap).Parse(string(content))
		if err != nil {
			return fmt.Errorf("error parsing template %s: %v", templateFile, err)
		}

		tp.templates[templateName] = tmpl
	}

	return nil
}

func (tp *TemplateProcessor) createDefaultTemplates(templatesDir string) error {
	defaultTemplates := map[string]string{
		"entidad_new.tpl": `version: "1.0"
method: POST
path: "/{{.EntityNamePlural}}/new"
description: "Crear nuevo {{.EntityName}}"

auth:
  required: true
  permissions: ["{{.EntityNamePlural}}.write"]

params:
  body:
    {{- range .Fields}}
    {{- if and (not .IsPrimaryKey) (not (isAuditField .Name))}}
    - name: "{{.NameSnake}}"
      type: "{{.Type}}"
      required: {{.IsRequired}}
      {{- if .Validation}}
      validation:
        {{- range $key, $value := .Validation}}
        {{$key}}: {{toString $value}}
        {{- end}}
      {{- end}}
      {{- if .Default}}
      default: {{toString .Default}}
      {{- end}}
      {{- if .Comment}}
      description: "{{.Comment}}"
      {{- end}}
    {{- end}}
    {{- end}}

commands:
  # Validaciones únicas si hay campos únicos
  {{- range .Fields}}
  {{- if and (not .IsPrimaryKey) (contains (toLowerCase .Name) "email")}}
  - type: validation
    sql: "SELECT COUNT(*) as count FROM {{$.TableName}} WHERE {{.Name}} = :{{.NameSnake}}"
    condition: "count > 0"
    on_true:
      action: stop
      http_code: 409
      message: "Ya existe un registro con ese {{.Name}}"
  {{- else if and (not .IsPrimaryKey) (contains (toLowerCase .Name) "username")}}
  - type: validation
    sql: "SELECT COUNT(*) as count FROM {{$.TableName}} WHERE {{.Name}} = :{{.NameSnake}}"
    condition: "count > 0"
    on_true:
      action: stop
      http_code: 409
      message: "Ya existe un registro con ese {{.Name}}"
  {{- end}}
  {{- end}}
  
  # Insertar {{.EntityName}}
  - type: exec
    sql: |
      INSERT INTO {{.TableName}} (
        {{- $fields := list}}
        {{- range .Fields}}
        {{- if and (not .IsPrimaryKey) (not (isAuditField .Name))}}
        {{- $fields = append $fields .Name}}
        {{- end}}
        {{- end}}
        {{- if .HasAuditFields}}
        {{- if gt (len $fields) 0}},{{end}}created_at, created_by
        {{- end}}
      ) VALUES (
        {{- $first := true}}
        {{- range .Fields}}
        {{- if and (not .IsPrimaryKey) (not (isAuditField .Name))}}
        {{- if $first}}{{$first = false}}{{else}},{{end}}
        :{{.NameSnake}}
        {{- end}}
        {{- end}}
        {{- if .HasAuditFields}}
        {{- if gt (len .Fields) 0}},{{end}}NOW(), :user_id
        {{- end}}
      )
      RETURNING *

response:
  success:
    code: 201
    message: "{{.EntityName}} creado exitosamente"
  error:
    code: 400
    message: "Error al crear {{.EntityName}}"
    sqlerror: true

hooks:
  after:
    - type: cache_invalidate
      keys: ["{{.EntityNamePlural}}:list:*", "{{.EntityNamePlural}}:search:*"]

audit:
  enabled: true
  log_params: true
  sensitive_fields: ["password", "token"]`,

		"entidad_update.tpl": `version: "1.0"
method: PUT
path: "/{{.EntityNamePlural}}/:id/update"
description: "Actualizar {{.EntityName}}"

auth:
  required: true
  permissions: ["{{.EntityNamePlural}}.write"]

params:
  path:
    - name: id
      type: {{range .PrimaryKeys}}{{if eq . "id"}}int{{else}}string{{end}}{{break}}{{else}}int{{end}}
      required: true
      validation:
        min: 1
      error_message: "Debe proporcionar ID válido"
  
  body:
    {{- range .Fields}}
    {{- if and (not .IsPrimaryKey) (not (isAuditField .Name)) (ne .Name "created_at") (ne .Name "created_by")}}
    - name: "{{.NameSnake}}"
      type: "{{.Type}}"
      required: false
      {{- if .Validation}}
      validation:
        {{- range $key, $value := .Validation}}
        {{$key}}: {{toString $value}}
        {{- end}}
      {{- end}}
      {{- if .Comment}}
      description: "{{.Comment}}"
      {{- end}}
    {{- end}}
    {{- end}}

commands:
  # Verificar que existe
  - type: validation
    sql: "SELECT COUNT(*) as count FROM {{.TableName}} WHERE {{range .PrimaryKeys}}{{.}} = :id{{break}}{{else}}id = :id{{end}}{{if .HasSoftDelete}} AND activo = true{{end}}"
    condition: "count = 0"
    on_true:
      action: stop
      http_code: 404
      message: "{{.EntityName}} no encontrado"
  
  # Actualizar
  - type: exec
    sql: |
      UPDATE {{.TableName}} SET
        {{- $first := true}}
        {{- range .Fields}}
        {{- if and (not .IsPrimaryKey) (not (isAuditField .Name)) (ne .Name "created_at") (ne .Name "created_by")}}
        {{if .NameSnake}} {{if hasField .NameSnake $.Fields}}
          {{if $first}}{{$first = false}}{{else}},{{end}}
          {{.Name}} = :{{.NameSnake}}
        {{end}}{{end}}
        {{- end}}
        {{- end}}
        {{- if .HasAuditFields}}
        {{if not $first}},{{end}}updated_at = NOW()
        {{- end}}
      WHERE {{range .PrimaryKeys}}{{.}} = :id{{break}}{{else}}id = :id{{end}}

response:
  success:
    code: 200
    message: "{{.EntityName}} actualizado exitosamente"

hooks:
  after:
    - type: cache_invalidate
      keys: ["{{.EntityNamePlural}}:list:*", "{{.EntityNamePlural}}:{{.id}}"]`,

		"entidad_delete.tpl": `version: "1.0"
method: DELETE
path: "/{{.EntityNamePlural}}/:id/delete"
description: "Eliminar {{.EntityName}} {{if .HasSoftDelete}}(soft delete){{else}}(físico){{end}}"

auth:
  required: true
  permissions: ["{{.EntityNamePlural}}.delete"]

params:
  path:
    - name: id
      type: {{range .PrimaryKeys}}{{if eq . "id"}}int{{else}}string{{end}}{{break}}{{else}}int{{end}}
      required: true
      validation:
        min: 1
      error_message: "Debe proporcionar ID válido"

commands:
  # Verificar que existe
  - type: validation
    sql: "SELECT COUNT(*) as count FROM {{.TableName}} WHERE {{range .PrimaryKeys}}{{.}} = :id{{break}}{{else}}id = :id{{end}}"
    condition: "count = 0"
    on_true:
      action: stop
      http_code: 404
      message: "{{.EntityName}} no encontrado"

  {{- if .HasSoftDelete}}
  # Verificar si ya está inactivo
  - type: validation
    sql: "SELECT COUNT(*) as count FROM {{.TableName}} WHERE {{range .PrimaryKeys}}{{.}} = :id{{break}}{{else}}id = :id{{end}} AND activo = true"
    condition: "count = 0"
    on_true:
      action: stop
      http_code: 404
      message: "{{.EntityName}} no encontrado o ya está inactivo"

  # Soft delete
  - type: exec
    sql: |
      UPDATE {{.TableName}} SET 
        activo = false, 
        deleted_at = NOW(),
        deleted_by = :user_id
      WHERE {{range .PrimaryKeys}}{{.}} = :id{{break}}{{else}}id = :id{{end}}
  {{- else}}
  # Delete físico
  - type: exec
    sql: |
      DELETE FROM {{.TableName}} 
      WHERE {{range .PrimaryKeys}}{{.}} = :id{{break}}{{else}}id = :id{{end}}
  {{- end}}

response:
  success:
    code: 200
    message: "{{.EntityName}} {{if .HasSoftDelete}}anulado{{else}}eliminado{{end}} exitosamente"

hooks:
  after:
    - type: cache_invalidate
      keys: ["{{.EntityNamePlural}}:list:*", "{{.EntityNamePlural}}:{{.id}}"]

audit:
  enabled: true`,

		"entidad_list.tpl": `version: "1.0"
method: GET
path: "/{{.EntityNamePlural}}/list"
description: "Lista de {{.EntityNamePlural}}"

auth:
  required: true
  permissions: ["{{.EntityNamePlural}}.read"]

params:
  query:
    - name: page
      type: int
      default: 1
      validation:
        min: 1
    - name: limit
      type: int
      default: 20
      validation:
        min: 1
        max: 100
    - name: search
      type: string
      required: false
    - name: orden
      type: string
      required: false
      default: "{{range .PrimaryKeys}}{{.}}{{break}}{{else}}id{{end}}"
      validation:
        pattern: "^[a-zA-Z_,]+$"

commands:
  - type: query
    sql: |
      SELECT *
      FROM {{.TableName}} 
      {{- if .HasSoftDelete}}
      WHERE activo = true 
        {{- if .search}}
        AND (
          {{- $first := true}}
          {{- range .Fields}}
          {{- if and (not .IsPrimaryKey) (eq .Type "string")}}
          {{if not $first}} OR {{end}}{{if $first}}{{$first = false}}{{end}}
          {{.Name}} ILIKE '%' || :search || '%'
          {{- end}}
          {{- end}}
        )
      {{- else if .search}}
      WHERE 
        {{- $first := true}}
        {{- range .Fields}}
        {{- if and (not .IsPrimaryKey) (eq .Type "string")}}
        {{if not $first}} OR {{end}}{{if $first}}{{$first = false}}{{end}}
        {{.Name}} ILIKE '%' || :search || '%'
        {{- end}}
        {{- end}}
      {{- end}}
      ORDER BY :orden ASC 
      LIMIT :limit OFFSET :offset
    returns: "multiple"
    transform_params:
      offset: "(:page - 1) * :limit"

response:
  structure:
    type: paginated
    pagination:
      total_query: |
        SELECT COUNT(*) 
        FROM {{.TableName}} 
        {{- if .HasSoftDelete}}
        WHERE activo = true 
          {{- if .search}}
          AND (
            {{- $first := true}}
            {{- range .Fields}}
            {{- if and (not .IsPrimaryKey) (eq .Type "string")}}
            {{if not $first}} OR {{end}}{{if $first}}{{$first = false}}{{end}}
            {{.Name}} ILIKE '%' || :search || '%'
            {{- end}}
            {{- end}}
          )
        {{- else if .search}}
        WHERE 
          {{- $first := true}}
          {{- range .Fields}}
          {{- if and (not .IsPrimaryKey) (eq .Type "string")}}
          {{if not $first}} OR {{end}}{{if $first}}{{$first = false}}{{end}}
          {{.Name}} ILIKE '%' || :search || '%'
          {{- end}}
          {{- end}}
        {{- end}}
  success_code: 200

cache:
  enabled: true
  ttl: 60
  key: "{{.EntityNamePlural}}:list:{{.page}}:{{.limit}}:{{.search}}"`,

		"entidad_get.tpl": `version: "1.0"
method: GET
path: "/{{.EntityNamePlural}}/:id/get"
description: "Obtener {{.EntityName}} por ID"

auth:
  required: true
  permissions: ["{{.EntityNamePlural}}.read"]

params:
  path:
    - name: id
      type: {{range .PrimaryKeys}}{{if eq . "id"}}int{{else}}string{{end}}{{break}}{{else}}int{{end}}
      required: true
      validation:
        min: 1
      error_message: "Debe proporcionar ID válido"

commands:
  # Obtener {{.EntityName}}
  - type: query
    sql: |
      SELECT *
      FROM {{.TableName}} 
      WHERE {{range .PrimaryKeys}}{{.}} = :id{{break}}{{else}}id = :id{{end}}
      {{- if .HasSoftDelete}}
      AND activo = true
      {{- end}}
    returns: "single"
    on_result:
      if_not_found:
        action: "abort"
        message: "{{.EntityName}} no encontrado"

response:
  success:
    code: 200
    message: "{{.EntityName}} obtenido exitosamente"
  error:
    code: 404
    message: "{{.EntityName}} no encontrado"
  
  {{- if .Includes}}
  includes:
    {{- range .Includes}}
    - relation: {{.Relation}}
      query: |
        SELECT * 
        FROM {{.ReferencedTable}} 
        WHERE {{.ReferencedColumn}} = :{{.ForeignKey}}
      type: {{.Type}}
    {{- end}}
  {{- end}}

cache:
  enabled: true
  ttl: 300
  key: "{{.EntityNamePlural}}:{{.id}}"

audit:
  enabled: true`,
	}

	for filename, content := range defaultTemplates {
		path := filepath.Join(templatesDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("error creating template %s: %v", filename, err)
		}
	}

	return nil
}

func (tp *TemplateProcessor) Process(templateName string, data interface{}) (string, error) {
	tmpl, exists := tp.templates[templateName]
	if !exists {
		return "", fmt.Errorf("template not found: %s", templateName)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("error executing template %s: %v", templateName, err)
	}

	return buf.String(), nil
}

func (tp *TemplateProcessor) ProcessToFile(templateName string, data interface{}, outputPath string) error {
	content, err := tp.Process(templateName, data)
	if err != nil {
		return err
	}

	// Crear directorio si no existe
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("error creating directory: %v", err)
	}

	// Renombrar archivo existente si existe
	if _, err := os.Stat(outputPath); err == nil {
		backupPath := fmt.Sprintf("%s.%s.bak", outputPath, time.Now().Format("20060102"))
		if err := os.Rename(outputPath, backupPath); err != nil {
			return fmt.Errorf("error backing up existing file: %v", err)
		}
	}

	// Escribir archivo
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("error writing file %s: %v", outputPath, err)
	}

	return nil
}

// Funciones helper para templates
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}

func toCamelCase(s string) string {
	words := strings.Split(strings.ToLower(s), "_")
	for i := 1; i < len(words); i++ {
		if len(words[i]) > 0 {
			words[i] = strings.ToUpper(words[i][:1]) + words[i][1:]
		}
	}
	return strings.Join(words, "")
}

func toPascalCase(s string) string {
	words := strings.Split(strings.ToLower(s), "_")
	for i := 0; i < len(words); i++ {
		if len(words[i]) > 0 {
			words[i] = strings.ToUpper(words[i][:1]) + words[i][1:]
		}
	}
	return strings.Join(words, "")
}

func pluralize(s string) string {
	if strings.HasSuffix(s, "y") {
		return strings.TrimSuffix(s, "y") + "ies"
	}
	if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") ||
		strings.HasSuffix(s, "z") || strings.HasSuffix(s, "ch") ||
		strings.HasSuffix(s, "sh") {
		return s + "es"
	}
	return s + "s"
}

func singularize(s string) string {
	if strings.HasSuffix(s, "ies") {
		return strings.TrimSuffix(s, "ies") + "y"
	}
	if strings.HasSuffix(s, "es") {
		s = strings.TrimSuffix(s, "es")
		if strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") ||
			strings.HasSuffix(s, "z") || strings.HasSuffix(s, "ch") ||
			strings.HasSuffix(s, "sh") {
			return s
		}
	}
	if strings.HasSuffix(s, "s") {
		return strings.TrimSuffix(s, "s")
	}
	return s
}

func formatType(dbType string) string {
	dbType = strings.ToLower(dbType)
	switch {
	case strings.Contains(dbType, "int"):
		if strings.Contains(dbType, "big") {
			return "int64"
		}
		return "int"
	case strings.Contains(dbType, "bool"):
		return "bool"
	case strings.Contains(dbType, "float") || strings.Contains(dbType, "double") || strings.Contains(dbType, "decimal") || strings.Contains(dbType, "numeric"):
		return "float"
	case strings.Contains(dbType, "timestamp") || strings.Contains(dbType, "datetime") || strings.Contains(dbType, "date"):
		return "string"
	case strings.Contains(dbType, "json"):
		return "object"
	case strings.Contains(dbType, "uuid"):
		return "string"
	default:
		return "string"
	}
}

func getValidation(col Column, isRequired bool) map[string]interface{} {
	validation := make(map[string]interface{})

	colName := strings.ToLower(col.Name)
	fieldType := formatType(col.DataType)

	// Validaciones por tipo de dato
	switch fieldType {
	case "string":
		// Longitud genérica
		if col.MaxLength != nil && *col.MaxLength > 0 {
			validation["min_length"] = 2
			validation["max_length"] = *col.MaxLength
		}
		colName = strings.ToLower(colName)
		// =====================
		// EMAIL
		// =====================
		if strings.Contains(colName, "email") || strings.Contains(colName, "correo") {
			validation["email"] = true
			// =====================
			// URL / WEB
			// =====================
		} else if strings.Contains(colName, "url") || strings.Contains(colName, "website") || strings.Contains(colName, "sitio") {
			validation["pattern"] = `^https?:\/\/[^\s/$.?#].[^\s]*$`
			// =====================
			// TELÉFONO
			// =====================
		} else if strings.Contains(colName, "telefono") || strings.Contains(colName, "phone") || strings.Contains(colName, "celular") || strings.Contains(colName, "movil") {
			validation["pattern"] = `^[0-9+\-\s\(\)]{7,20}$`
			// =====================
			// CUIT / CUIL (Argentina)
			// =====================
		} else if strings.Contains(colName, "cuit") {
			validation["pattern"] = `^(20|23|24|27|30|33|34)-?\d{8}-?\d$`
		} else if strings.Contains(colName, "cuil") {
			validation["pattern"] = `^(20|23|24|27)-?\d{8}-?\d$`
			// =====================
			// CBU (Argentina)
			// =====================
		} else if strings.Contains(colName, "cbu") {
			validation["pattern"] = `^\d{22}$`
			// =====================
			// CVU / ALIAS (Argentina)
			// =====================
		} else if strings.Contains(colName, "cvu") {
			validation["pattern"] = `^\d{22}$`
		} else if strings.Contains(colName, "alias") && (strings.Contains(colName, "bancario") || strings.Contains(colName, "cvu")) {
			validation["pattern"] = `^[a-z0-9.]{6,20}$`
			// =====================
			// DNI
			// =====================
		} else if colName == "dni" || strings.Contains(colName, "documento") {
			validation["pattern"] = `^\d{7,8}$`
			// =====================
			// PASAPORTE
			// =====================
		} else if strings.Contains(colName, "pasaporte") || strings.Contains(colName, "passport") {
			validation["pattern"] = `^[A-Z]{3}\d{6}$`
			// =====================
			// RUC (Perú, Paraguay)
			// =====================
		} else if strings.Contains(colName, "ruc") {
			validation["pattern"] = `^\d{11}$`
			// =====================
			// RFC (México)
			// =====================
		} else if strings.Contains(colName, "rfc") {
			validation["pattern"] = `^[A-ZÑ&]{3,4}\d{6}[A-Z0-9]{3}$`
			// =====================
			// CURP (México)
			// =====================
		} else if strings.Contains(colName, "curp") {
			validation["pattern"] = `^[A-Z]{4}\d{6}[HM][A-Z]{5}[0-9A-Z]\d$`
			// =====================
			// CPF (Brasil)
			// =====================
		} else if strings.Contains(colName, "cpf") {
			validation["pattern"] = `^\d{3}\.\d{3}\.\d{3}-\d{2}$|^\d{11}$`
			// =====================
			// CNPJ (Brasil)
			// =====================
		} else if strings.Contains(colName, "cnpj") {
			validation["pattern"] = `^\d{2}\.\d{3}\.\d{3}\/\d{4}-\d{2}$|^\d{14}$`
			// =====================
			// RUT (Chile)
			// =====================
		} else if strings.Contains(colName, "rut") {
			validation["pattern"] = `^\d{1,2}\.\d{3}\.\d{3}-[\dkK]$|^\d{7,8}-[\dkK]$`
			// =====================
			// CÓDIGO POSTAL
			// =====================
		} else if strings.Contains(colName, "codigo_postal") || strings.Contains(colName, "postal") || strings.Contains(colName, "zip") {
			validation["pattern"] = `^([A-Z]\d{4}[A-Z]{3}|\d{4,5})$`
			// =====================
			// USERNAME
			// =====================
		} else if strings.Contains(colName, "username") || strings.Contains(colName, "usuario") || strings.Contains(colName, "login") {
			validation["pattern"] = `^[a-zA-Z0-9._-]{4,20}$`
			// =====================
			// NOMBRE / APELLIDO
			// =====================
		} else if strings.Contains(colName, "nombre") || strings.Contains(colName, "apellido") || strings.Contains(colName, "name") {
			validation["pattern"] = `^[\p{L}\s]+$`
			//validation["pattern"] = `^[A-Za-zÁÉÍÓÚáéíóúÑñÃãÕõÇçÊêÔôÂâÀà\s]+$`
			// =====================
			// PASSWORD
			// =====================
		} else if strings.Contains(colName, "password") || strings.Contains(colName, "clave") || strings.Contains(colName, "contrasena") {
			validation["pattern"] = `^[A-Za-z0-9@$!%*?&#+._-]{8,}$`
			// =====================
			// UUID
			// =====================
		} else if strings.Contains(colName, "uuid") || strings.Contains(colName, "guid") {
			validation["pattern"] = `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`
			// =====================
			// IP v4
			// =====================
		} else if strings.Contains(colName, "ip") && !strings.Contains(colName, "v6") {
			validation["pattern"] = `^(25[0-5]|2[0-4]\d|[01]?\d\d?)\.((25[0-5]|2[0-4]\d|[01]?\d\d?)\.){2}(25[0-5]|2[0-4]\d|[01]?\d\d?)$`
			// =====================
			// IP v6
			// =====================
		} else if strings.Contains(colName, "ipv6") {
			validation["pattern"] = `^(([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}|::([0-9a-fA-F]{1,4}:){0,6}[0-9a-fA-F]{1,4})$`
			// =====================
			// MAC ADDRESS
			// =====================
		} else if strings.Contains(colName, "mac") && strings.Contains(colName, "address") {
			validation["pattern"] = `^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`
			// =====================
			// PATENTE ARGENTINA
			// =====================
		} else if strings.Contains(colName, "patente") || strings.Contains(colName, "dominio") {
			validation["pattern"] = `^([A-Z]{3}\d{3}|[A-Z]{2}\d{3}[A-Z]{2})$`
			// =====================
			// VIN (Vehicle Identification Number)
			// =====================
		} else if strings.Contains(colName, "vin") || strings.Contains(colName, "chasis") {
			validation["pattern"] = `^[A-HJ-NPR-Z0-9]{17}$`
			// =====================
			// COLOR HEX
			// =====================
		} else if strings.Contains(colName, "color") && !strings.Contains(colName, "nombre") {
			validation["pattern"] = `^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`
			// =====================
			// TARJETA DE CRÉDITO
			// =====================
		} else if strings.Contains(colName, "tarjeta") || strings.Contains(colName, "card") {
			validation["pattern"] = `^\d{13,19}$`
			// =====================
			// CVV
			// =====================
		} else if strings.Contains(colName, "cvv") || strings.Contains(colName, "cvc") {
			validation["pattern"] = `^\d{3,4}$`
			// =====================
			// IBAN (Internacional)
			// =====================
		} else if strings.Contains(colName, "iban") {
			validation["pattern"] = `^[A-Z]{2}\d{2}[A-Z0-9]{1,30}$`
			// =====================
			// SWIFT/BIC
			// =====================
		} else if strings.Contains(colName, "swift") || strings.Contains(colName, "bic") {
			validation["pattern"] = `^[A-Z]{6}[A-Z0-9]{2}([A-Z0-9]{3})?$`
			// =====================
			// ISBN (10 o 13 dígitos)
			// =====================
		} else if strings.Contains(colName, "isbn") {
			validation["pattern"] = `^(97[89])?\d{9}[\dX]$`
			// =====================
			// SLUG (URL friendly)
			// =====================
		} else if strings.Contains(colName, "slug") {
			validation["pattern"] = `^[a-z0-9]+(?:-[a-z0-9]+)*$`
			// =====================
			// COORDENADAS (Latitud)
			// =====================
		} else if strings.Contains(colName, "latitud") || strings.Contains(colName, "latitude") || colName == "lat" {
			validation["pattern"] = `^-?([0-8]?\d(\.\d+)?|90(\.0+)?)$`
			// =====================
			// COORDENADAS (Longitud)
			// =====================
		} else if strings.Contains(colName, "longitud") || strings.Contains(colName, "longitude") || colName == "lng" || colName == "lon" {
			validation["pattern"] = `^-?(1[0-7]\d(\.\d+)?|180(\.0+)?|\d{1,2}(\.\d+)?)$`
			// =====================
			// HASHTAG
			// =====================
		} else if strings.Contains(colName, "hashtag") || strings.Contains(colName, "tag") {
			validation["pattern"] = `^#[A-Za-z0-9_]+$`
			// =====================
			// HANDLE SOCIAL (@usuario)
			// =====================
		} else if strings.Contains(colName, "handle") || strings.Contains(colName, "twitter") || strings.Contains(colName, "instagram") {
			validation["pattern"] = `^@?[A-Za-z0-9_]{1,15}$`
			// =====================
			// VERSIÓN SEMÁNTICA
			// =====================
		} else if strings.Contains(colName, "version") {
			validation["pattern"] = `^\d+\.\d+\.\d+(-[a-zA-Z0-9]+)?$`
			// =====================
			// MONEDA (código ISO)
			// =====================
		} else if strings.Contains(colName, "moneda") || strings.Contains(colName, "currency") {
			validation["pattern"] = `^[A-Z]{3}$`
			// =====================
			// CÓDIGO DE IDIOMA (ISO 639-1)
			// =====================
		} else if strings.Contains(colName, "idioma") || strings.Contains(colName, "language") || strings.Contains(colName, "lang") {
			validation["pattern"] = `^[a-z]{2}(-[A-Z]{2})?$`
			// =====================
			// CÓDIGO DE PAÍS (ISO 3166-1 alpha-2)
			// =====================
		} else if strings.Contains(colName, "pais") || strings.Contains(colName, "country") {
			validation["pattern"] = `^[A-Z]{2}$`
			// =====================
			// HORA (HH:MM o HH:MM:SS)
			// =====================
		} else if strings.Contains(colName, "hora") || strings.Contains(colName, "time") {
			validation["pattern"] = `^([01]\d|2[0-3]):[0-5]\d(:[0-5]\d)?$`
			// =====================
			// NÚMERO DE ORDEN / FACTURA
			// =====================
		} else if strings.Contains(colName, "numero") && (strings.Contains(colName, "orden") || strings.Contains(colName, "factura") || strings.Contains(colName, "invoice")) {
			validation["pattern"] = `^[A-Z0-9]{5,20}$`
			// =====================
			// MATRÍCULA (estudiantes)
			// =====================
		} else if strings.Contains(colName, "matricula") || strings.Contains(colName, "legajo") {
			validation["pattern"] = `^[A-Z0-9]{5,15}$`
			// =====================
			// CÓDIGO DE BARRAS (EAN-13)
			// =====================
		} else if strings.Contains(colName, "codigo_barras") || strings.Contains(colName, "ean") || strings.Contains(colName, "barcode") {
			validation["pattern"] = `^\d{13}$`
			// =====================
			// NÚMERO DE LOTE
			// =====================
		} else if strings.Contains(colName, "lote") || strings.Contains(colName, "batch") {
			validation["pattern"] = `^[A-Z0-9]{6,20}$`
			// =====================
			// EXTENSIÓN DE ARCHIVO
			// =====================
		} else if strings.Contains(colName, "extension") {
			validation["pattern"] = `^\.[a-z0-9]{2,5}$`
			// =====================
			// MIME TYPE
			// =====================
		} else if strings.Contains(colName, "mime") {
			validation["pattern"] = `^[a-z]+\/[a-z0-9\-\+\.]+$`
			// =====================
			// TWITTER/X POST ID
			// =====================
		} else if strings.Contains(colName, "tweet") && strings.Contains(colName, "id") {
			validation["pattern"] = `^\d{15,20}$`
			// =====================
			// YOUTUBE VIDEO ID
			// =====================
		} else if strings.Contains(colName, "youtube") && strings.Contains(colName, "id") {
			validation["pattern"] = `^[A-Za-z0-9_-]{11}$`
			// =====================
			// TEXTO GENERAL
			// =====================
		} else {
			validation["pattern"] = `^.{1,255}$`
		}
	case "int", "int32":
		validation["min"] = -2147483648
		validation["max"] = 2147483647
	case "int64":
		validation["min"] = -9223372036854775808
		validation["max"] = 9223372036854775807
	case "uint", "uint32":
		validation["min"] = 0
		validation["max"] = 4294967295
	case "uint64":
		validation["min"] = 0
		validation["max"] = 1844674407370955169
	case "float32", "float64":
		validation["min"] = -1e12
		validation["max"] = 1e12
	case "bool":
		validation["type"] = "boolean"
	case "date":
		validation["pattern"] = `^\d{4}-\d{2}-\d{2}$`
	case "datetime", "timestamp":
		validation["pattern"] = `^\d{4}-\d{2}-\d{2}T([01]\d|2[0-3]):[0-5]\d(:[0-5]\d)?$`
	case "time":
		validation["pattern"] = `^([01]\d|2[0-3]):[0-5]\d(:[0-5]\d)?$`
	case "year":
		validation["pattern"] = `^\d{4}$`
		validation["min"] = 1900
		validation["max"] = 2100
	}

	// Validaciones especiales por nombre de campo
	if strings.Contains(colName, "password") {
		validation["min_length"] = 8
	}

	return validation
}

func getDefault(col Column, fieldType string) interface{} {
	if col.DefaultValue != nil {
		defaultStr := *col.DefaultValue

		// Limpiar valores de funciones de base de datos
		if strings.Contains(strings.ToLower(defaultStr), "nextval") ||
			strings.Contains(strings.ToLower(defaultStr), "now()") ||
			strings.Contains(strings.ToLower(defaultStr), "current_timestamp") {
			return nil
		}

		// Remover comillas simples
		defaultStr = strings.Trim(defaultStr, "'")

		switch fieldType {
		case "bool":
			if strings.Contains(strings.ToLower(defaultStr), "true") || defaultStr == "1" {
				return true
			}
			return false
		case "int":
			if num, err := strconv.Atoi(defaultStr); err == nil {
				return num
			}
		case "int64":
			if num, err := strconv.ParseInt(defaultStr, 10, 64); err == nil {
				return num
			}
		case "float":
			if num, err := strconv.ParseFloat(defaultStr, 64); err == nil {
				return num
			}
		default:
			return defaultStr
		}
	}

	return nil
}

func getDefaultValue(col Column) interface{} {
	fieldType := formatType(col.DataType)
	return getDefault(col, fieldType)
}

func toString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strconv.Quote(v)
	case int, int64, float32, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%v", v)
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func hasField(fieldName string, fields []map[string]interface{}) bool {
	for _, field := range fields {
		if name, ok := field["NameSnake"].(string); ok && name == fieldName {
			return true
		}
	}
	return false
}

func isAuditField(fieldName string) bool {
	fieldName = strings.ToLower(fieldName)
	auditFields := []string{
		"created_at", "created_by",
		"updated_at", "updated_by",
		"deleted_at", "deleted_by",
	}

	for _, auditField := range auditFields {
		if fieldName == auditField {
			return true
		}
	}
	return false
}

// shouldIncludeInUpdate determina si un campo debe incluirse en el update
func shouldIncludeInUpdate(fieldName string) bool {
	fieldName = strings.ToLower(fieldName)

	// Campos que NO deben incluirse en update
	excludedFields := []string{
		"id", "created_at", "created_by",
		"deleted_at", "deleted_by",
	}

	for _, excluded := range excludedFields {
		if fieldName == excluded {
			return false
		}
	}

	return true
}

// coalesceFunc devuelve la función COALESCE para SQL
func coalesceFunc() string {
	return "COALESCE"
}

// nowFunc devuelve la función NOW() para SQL
func nowFunc() string {
	return "NOW()"
}

// list crea una lista vacía para usar en templates
func list() []interface{} {
	return []interface{}{}
}

// appendFunc agrega un elemento a una lista
func appendFunc(list []interface{}, item interface{}) []interface{} {
	return append(list, item)
}

// greaterThan verifica si a > b
func greaterThan(a, b int) bool {
	return a > b
}

// addInt returns a + b (used as "add" in templates).
func addInt(a, b int) int {
	return a + b
}

// shouldIncludeInParams determina si un campo debe incluirse en los parámetros
func shouldIncludeInParams(fieldName string, isPrimaryKey bool) bool {
	fieldName = strings.ToLower(fieldName)

	// Campos que NO deben incluirse en parámetros
	excludedFields := []string{
		"created_at", "updated_at", "deleted_at",
		"created_by", "updated_by", "deleted_by",
	}

	// No incluir claves primarias en body params (van en path)
	if isPrimaryKey {
		return false
	}

	for _, excluded := range excludedFields {
		if fieldName == excluded {
			return false
		}
	}

	return true
}

func indentFunc(spaces int, v string) string {
	pad := strings.Repeat(" ", spaces)
	return strings.ReplaceAll(v, "\n", "\n"+pad)
}

// defaultValue returns `val` if it is non-empty/non-zero, otherwise returns `def`.
// This mimics the Sprig/Helm `default` function so templates can use:
//
//	{{ .SomeField | default "fallback" }}
func defaultValue(def interface{}, val interface{}) interface{} {
	if isEmpty(val) {
		return def
	}
	return val
}

// isEmpty reports whether a value is its zero-value (nil, "", 0, false, empty slice/map).
func isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case bool:
		return !val
	case int:
		return val == 0
	case int32:
		return val == 0
	case int64:
		return val == 0
	case float32:
		return val == 0
	case float64:
		return val == 0
	case []interface{}:
		return len(val) == 0
	case map[string]interface{}:
		return len(val) == 0
	}
	return false
}
