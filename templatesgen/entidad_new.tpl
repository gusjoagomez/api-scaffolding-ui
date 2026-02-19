version: "1.0"
method: POST
path: "/{{.TableNameLower}}/new"
description: "Crear nuevo {{.EntityName}}"

auth:
  required: true
  permissions: ["{{.TableNameLower}}.write"]

params:
  body:
    {{- $hasPassword := false }}
    {{- range .Fields}}
    {{- if and (not .IsPrimaryKey) (ne .Name "created_at") (ne .Name "updated_at") (ne .Name "deleted_at") (ne .Name "deleted_by") (ne .Name "created_by") (ne .Name "updated_by")}}

    - name: {{.NameSnake}}
      type: {{.Type}}
      required: {{.IsRequired}}
      {{- if .Validation}}
      validation:
        {{- range $key, $value := .Validation}}
        {{- if (eq $key "pattern")}}
        {{$key}}: '{{printf "%v" $value}}'
        {{- else}}
        {{$key}}: {{printf "%v" $value}}
        {{- end}}
        {{- end}}
      {{- if or (eq .Name "password") (eq .Name "password_hash")}}
      constraints: [has_lower, has_upper, has_number, has_special]
      {{- $hasPassword = true }}
      {{- end}}
      {{- end}}
      {{- if .Default}}
      default: "{{printf "%v" .Default}}"
      {{- end}}
      error_message: "{{.NamePascal}} inválido"
    {{- end}}
    {{- end}}

commands:
  ## Validar CODIGO único
  #- type: validation
  #  sql: "SELECT COUNT(*) as count FROM {{.TableName}} WHERE code = :code"
  #  condition: "count > 0"
  #  on_true:
  #    action: stop
  #    http_code: 409
  #    message: "Ya existe ese codigo para la {{.EntityName}}"

  # Insertar {{.EntityName}}
  - type: exec
    sql: |
      INSERT INTO {{.TableName}} (
        {{- $first := true}}
        {{- range .Fields}}
        {{- if and (not .IsPrimaryKey) (ne .Name "created_at") (ne .Name "updated_at") (ne .Name "deleted_at") (ne .Name "deleted_by")}}
        {{- if $first}}{{$first = false}}{{else}},{{end}}
        {{.Name}}
        {{- end}}
        {{- end}}
        {{- if .HasAuditFields}}
        ,created_at, created_by
        {{- end}}
      ) VALUES (
        {{- $first := true}}
        {{- range .Fields}}
        {{- if and (not .IsPrimaryKey) (ne .Name "created_at") (ne .Name "updated_at") (ne .Name "deleted_at") (ne .Name "deleted_by")}}
        {{- if $first}}{{$first = false}}{{else}},{{end}}
        :{{.NameSnake}}
        {{- end}}
        {{- end}}
        {{- if .HasAuditFields}}
        ,NOW(), :user_id
        {{- end}}
      )
      RETURNING *
      {{- if $hasPassword}}
    transform_params:
      password_hash: "hash_password(:password)"
      {{- end}}

response:
  success:
    code: 201
    message: "{{.EntityName}} creado exitosamente"
  error:
    code: 400
    message: "Error al crear {{.EntityName}}"
    sqlerror: true

map:
  {{- range .Fields}}
  {{- if and (ne .Name "password_hash") (ne .Name "password") (ne .Name "created_at") (ne .Name "updated_at") (ne .Name "deleted_at") (ne .Name "deleted_by")}}
  {{.Name}}: "{{.NameSnake}}"
  {{- end}}
  {{- end}}
  {{- if .HasAuditFields}}
  created_at: "created_at"
  {{- end}}


hooks:
  after:
    - type: cache_invalidate
      keys: ["{{.TableNameLower}}:list:*", "{{.TableNameLower}}:search:*"]
    - type: notification
      event: "{{.TableNameLower}}.creado"

audit:
  enabled: true
  log_params: true
  sensitive_fields: []
  exclude_params: []

cache:
  enabled: false
