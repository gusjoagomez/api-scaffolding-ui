package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"api-scaffolding/internal/database"
	"api-scaffolding/internal/utils"
)

type Generator struct {
	config            *Config
	dbScanner         *database.Scanner
	templateProcessor *TemplateProcessor
	allTables         []database.Table
}

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

func NewGenerator(config *Config, dbScanner *database.Scanner, templateProcessor *TemplateProcessor) *Generator {
	return &Generator{
		config:            config,
		dbScanner:         dbScanner,
		templateProcessor: templateProcessor,
	}
}

func (g *Generator) Generate() error {
	fmt.Println("Starting scaffolding generation...")

	// Obtener tablas de la base de datos
	tables, err := g.dbScanner.GetTables(g.config.ProjectSchema, g.config.ProjectTables)
	if err != nil {
		return fmt.Errorf("error getting tables: %v", err)
	}

	g.allTables = tables

	fmt.Printf("Found %d tables in schema %s\n", len(tables), g.config.ProjectSchema)

	// Generar archivos para cada tabla
	for _, table := range tables {
		if err := g.generateTableFiles(table); err != nil {
			fmt.Printf("Warning: error generating files for table %s: %v\n", table.Name, err)
			continue
		}
	}

	fmt.Println("Scaffolding generation completed successfully!")
	return nil
}

func (g *Generator) generateTableFiles(table database.Table) error {
	tableName := strings.ToLower(table.Name)
	entityName := singularize(tableName)

	// Crear directorio para la entidad
	entityDir := filepath.Join(g.config.ProjectDir, tableName)
	if err := os.MkdirAll(entityDir, 0755); err != nil {
		return fmt.Errorf("error creating directory %s: %v", entityDir, err)
	}

	// Preparar datos para templates
	templateData := g.prepareTemplateData(table)

	// Generar archivos basados en templates disponibles
	templates := []string{
		"entidad_new.tpl",
		"entidad_update.tpl",
		"entidad_delete.tpl",
		"entidad_list.tpl",
		"entidad_get.tpl",
	}

	for _, templateName := range templates {
		// Verificar si el template existe
		if _, err := os.Stat(filepath.Join("templates", templateName)); os.IsNotExist(err) {
			// Crear template por defecto si no existe
			if err := g.createDefaultTemplate(templateName); err != nil {
				fmt.Printf("Warning: template %s not found and could not create default: %v\n", templateName, err)
				continue
			}
		}

		// Determinar nombre del archivo de salida
		outputFile := g.getOutputFilename(templateName, entityName)
		outputPath := filepath.Join(entityDir, outputFile)

		// Procesar template
		if err := g.templateProcessor.ProcessToFile(templateName, templateData, outputPath); err != nil {
			return fmt.Errorf("error processing template %s: %v", templateName, err)
		}

		fmt.Printf("Generated: %s\n", outputPath)
	}

	return nil
}

func (g *Generator) prepareTemplateData(table database.Table) map[string]interface{} {
	data := make(map[string]interface{})

	// Información básica de la tabla
	data["TableName"] = table.Name
	data["TableNameLower"] = strings.ToLower(table.Name)
	data["EntityName"] = singularize(table.Name)
	data["EntityNameTitle"] = utils.TitleFirst(table.Name)
	data["EntityNameLower"] = strings.ToLower(singularize(table.Name))
	data["EntityNamePlural"] = pluralize(strings.ToLower(table.Name))
	data["Schema"] = table.Schema

	// Convertir columnas de database a generator
	fields := make([]map[string]interface{}, len(table.Columns))
	for i, dbCol := range table.Columns {
		// Convertir database.Column a generator.Column
		col := Column{
			Name:         dbCol.Name,
			DataType:     dbCol.DataType,
			IsNullable:   dbCol.IsNullable,
			IsPrimaryKey: dbCol.IsPrimaryKey,
			IsForeignKey: dbCol.IsForeignKey,
			DefaultValue: dbCol.DefaultValue,
			MaxLength:    dbCol.MaxLength,
			Comment:      dbCol.Comment,
		}

		fieldType := formatType(col.DataType)
		isRequired := !col.IsNullable && col.DefaultValue == nil

		field := map[string]interface{}{
			"Name":         col.Name,
			"NameSnake":    toSnakeCase(col.Name),
			"NameCamel":    toCamelCase(col.Name),
			"NamePascal":   toPascalCase(col.Name),
			"Type":         fieldType,
			"DBType":       col.DataType,
			"IsRequired":   isRequired,
			"IsPrimaryKey": g.isPrimaryKey(col.Name, table.PrimaryKeys),
			"IsForeignKey": col.IsForeignKey,
			"Validation":   getValidation(col, isRequired),
			"Default":      getDefault(col, fieldType),
			"MaxLength":    col.MaxLength,
			"Comment":      col.Comment,
		}

		fields[i] = field
	}

	data["Fields"] = fields
	data["PrimaryKeys"] = table.PrimaryKeys

	// Convertir foreign keys de database a generator
	var foreignKeys []ForeignKey
	for _, dbFk := range table.ForeignKeys {
		fk := ForeignKey{
			ColumnName:       dbFk.ColumnName,
			ReferencedTable:  dbFk.ReferencedTable,
			ReferencedColumn: dbFk.ReferencedColumn,
			ConstraintName:   dbFk.ConstraintName,
		}
		foreignKeys = append(foreignKeys, fk)
	}
	data["ForeignKeys"] = foreignKeys

	// Determinar si tiene campos de auditoría
	data["HasAuditFields"] = g.hasAuditFields(table.Columns)
	data["HasSoftDelete"] = g.hasSoftDeleteField(table.Columns)

	// Configuración del proyecto
	data["ProjectConfig"] = g.config

	// Relaciones para includes
	if g.shouldIncludeRelations(table.Name) {
		data["Includes"] = g.prepareIncludes(table)
	}

	return data
}

// Asegurarse de que estas funciones estén definidas
func (g *Generator) isPrimaryKey(columnName string, primaryKeys []string) bool {
	for _, pk := range primaryKeys {
		if strings.EqualFold(pk, columnName) {
			return true
		}
	}
	return false
}

func (g *Generator) hasAuditFields(columns []database.Column) bool {
	auditFields := []string{"created_at", "created_by", "updated_at", "updated_by"}
	for _, col := range columns {
		colName := strings.ToLower(col.Name)
		for _, auditField := range auditFields {
			if colName == auditField {
				return true
			}
		}
	}
	return false
}

func (g *Generator) hasSoftDeleteField(columns []database.Column) bool {
	softDeleteFields := []string{"deleted_at", "deleted_by", "activo", "is_active", "status", "deleted"}
	for _, col := range columns {
		colName := strings.ToLower(col.Name)
		for _, field := range softDeleteFields {
			if colName == field {
				return true
			}
		}
	}
	return false
}

func (g *Generator) isForeignKey(columnName string, foreignKeys []database.ForeignKey) bool {
	for _, fk := range foreignKeys {
		if strings.EqualFold(fk.ColumnName, columnName) {
			return true
		}
	}
	return false
}

func (g *Generator) shouldIncludeRelations(tableName string) bool {
	if len(g.config.ProjectRelations) == 0 {
		return false
	}

	if len(g.config.ProjectRelations) == 1 && g.config.ProjectRelations[0] == "*" {
		return true
	}

	for _, relation := range g.config.ProjectRelations {
		if strings.EqualFold(relation, tableName) {
			return true
		}
	}

	return false
}

func (g *Generator) prepareIncludes(table database.Table) []map[string]interface{} {
	var includes []map[string]interface{}
	existingRelations := make(map[string]bool)

	// 1. Relaciones Salientes (BelongsTo - 1:1)
	for _, fk := range table.ForeignKeys {
		refTable := strings.ToLower(fk.ReferencedTable)
		// Usually we use singular for object relations
		relation := singularize(refTable)

		if existingRelations[relation] {
			continue
		}

		include := map[string]interface{}{
			"Relation":         relation,
			"ForeignKey":       fk.ColumnName,
			"ReferencedTable":  fk.ReferencedTable,
			"ReferencedColumn": fk.ReferencedColumn,
			"Type":             "object",
			"Query":            fmt.Sprintf(`SELECT * FROM %s WHERE %s = :%s`, fk.ReferencedTable, fk.ReferencedColumn, fk.ColumnName),
		}
		includes = append(includes, include)
		existingRelations[relation] = true
	}

	// 2. Relaciones Entrantes (HasMany - 1:N) y Join Tables (Many-to-Many - N:M)
	// Iterate through all other tables to find those that point to the current table
	for _, otherTable := range g.allTables {
		if strings.EqualFold(otherTable.Name, table.Name) {
			continue
		}

		// Find FKs in otherTable that point to my table (table.Name)
		for _, fk := range otherTable.ForeignKeys {
			if strings.EqualFold(fk.ReferencedTable, table.Name) {
				// Found incoming reference (otherTable -> table)

				// Case A: Direct 1:N Relation (e.g. User -> Posts)
				// otherTable (Posts) has user_id.
				// We add "posts" to User.
				directRelationName := pluralize(strings.ToLower(otherTable.Name))

				// Heuristic for Join Table:
				// If otherTable has > 1 FK, it might be a join table (e.g. user_roles).
				// We will check for indirect targets as well.
				isPotentialJoinTable := len(otherTable.ForeignKeys) > 1

				// We add the direct relation UNLESS it looks like a join table and we assume user prefers the target.
				// However, safely, we should probably add the direct relation too unless we are sure.
				// For 'user_roles', seeing a list of 'user_roles' objects is technically correct but maybe not what they want if 'roles' is available.
				// Let's add it if it doesn't conflict.

				if !existingRelations[directRelationName] {
					include := map[string]interface{}{
						"Relation":         directRelationName,
						"ForeignKey":       "id", // My PK (assuming id)
						"ReferencedTable":  otherTable.Name,
						"ReferencedColumn": fk.ColumnName, // The FK column in the other table
						"Type":             "array",
						"Query":            fmt.Sprintf(`SELECT * FROM %s WHERE %s = :id`, otherTable.Name, fk.ColumnName),
					}
					includes = append(includes, include)
					existingRelations[directRelationName] = true
				}

				// Case B: Indirect N:M Relation (via Join Table)
				// If otherTable has another FK to a third table.
				if isPotentialJoinTable {
					for _, otherFK := range otherTable.ForeignKeys {
						// Skip the FK pointing to me
						if otherFK.ColumnName == fk.ColumnName {
							continue
						}

						// This is the FK to the Target
						targetTableName := otherFK.ReferencedTable
						// Relation name usually plural of target table
						targetRelationName := pluralize(strings.ToLower(targetTableName))

						if !existingRelations[targetRelationName] {
							// Construct N:M Query

							// Find target table PK
							var targetPK string = "id" // Default
							// Try to see if we can find the exact PK name if we have the table info
							// But we are iterating over allTables, so we can find it.
							for _, t := range g.allTables {
								if strings.EqualFold(t.Name, targetTableName) {
									if len(t.PrimaryKeys) > 0 {
										targetPK = t.PrimaryKeys[0]
									}
									break
								}
							}

							// Query: SELECT t.* FROM target t JOIN join_table jt ON t.id = jt.target_id WHERE jt.source_id = :id
							// Aliases: t (target), jt (join table)
							// Note: User example used SELECT r.id, r.nombre ... FROM roles r.
							// We will use SELECT t.* for generic scaffold.

							query := fmt.Sprintf(`SELECT t.* FROM %s t JOIN %s jt ON t.%s = jt.%s WHERE jt.%s = :id`,
								targetTableName,
								otherTable.Name,
								targetPK,
								otherFK.ColumnName,
								fk.ColumnName)

							include := map[string]interface{}{
								"Relation":         targetRelationName,
								"ForeignKey":       "id",
								"ReferencedTable":  targetTableName,
								"ReferencedColumn": "N/A (Many-to-Many)",
								"Type":             "array",
								"Query":            query,
							}
							includes = append(includes, include)
							existingRelations[targetRelationName] = true
						}
					}
				}
			}
		}
	}

	return includes
}

func (g *Generator) getOutputFilename(templateName, entityName string) string {
	baseName := strings.TrimSuffix(templateName, ".tpl")

	// Reemplazar "entidad" con el nombre real de la entidad
	filename := strings.Replace(baseName, "entidad", entityName, 1)

	// Agregar extensión basada en configuración
	switch g.config.ProjectFileTypes {
	case "yaml", "yml":
		return filename + ".yaml"
	case "json":
		return filename + ".json"
	case "api":
		return filename + ".api"
	default:
		return filename + ".yaml"
	}
}

func (g *Generator) createDefaultTemplate(templateName string) error {
	templatesDir := "templates"
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return err
	}

	// Crear templates por defecto básicos
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
    {{- if and (not .IsPrimaryKey) (ne .Name "created_at") (ne .Name "updated_at") (ne .Name "deleted_at")}}
    - name: "{{.NameSnake}}"
      type: "{{.Type}}"
      required: {{.IsRequired}}
      {{- if .Validation}}
      validation:
        {{- range $key, $value := .Validation}}
        {{$key}}: {{$value}}
        {{- end}}
      {{- end}}
      {{- if .Default}}
      default: {{.Default}}
      {{- end}}
    {{- end}}
    {{- end}}

commands:
  # Insertar registro
  - type: exec
    sql: |
      INSERT INTO {{.TableName}} (
        {{- $first := true}}
        {{- range .Fields}}
        {{- if and (not .IsPrimaryKey) (ne .Name "created_at") (ne .Name "updated_at") (ne .Name "deleted_at")}}
        {{- if $first}}{{$first = false}}{{else}},{{end}}
        {{.Name}}
        {{- end}}
        {{- end}}
      ) VALUES (
        {{- $first := true}}
        {{- range .Fields}}
        {{- if and (not .IsPrimaryKey) (ne .Name "created_at") (ne .Name "updated_at") (ne .Name "deleted_at")}}
        {{- if $first}}{{$first = false}}{{else}},{{end}}
        :{{.NameSnake}}
        {{- end}}
        {{- end}}
      )
      RETURNING *`,

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
    - name: limit
      type: int
      default: 20

commands:
  - type: query
    sql: |
      SELECT * 
      FROM {{.TableName}} 
      {{- if .HasSoftDelete}}
      WHERE activo = true 
      {{- end}}
      ORDER BY {{range .PrimaryKeys}}{{.}}{{break}}{{else}}id{{end}} ASC 
      LIMIT :limit OFFSET :offset
    transform_params:
      offset: "(:page - 1) * :limit"

  {{- if .Includes}}
  includes:
    {{- range .Includes}}
    - relation: {{.Relation}}
      query: |
        {{.Query | indent 8}}
      type: {{.Type}}
    {{- end}}
  {{- end}}`,

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
        {{.Query | indent 8}}
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

	content, exists := defaultTemplates[templateName]
	if !exists {
		return fmt.Errorf("no default template for %s", templateName)
	}

	path := filepath.Join(templatesDir, templateName)
	return os.WriteFile(path, []byte(content), 0644)
}
