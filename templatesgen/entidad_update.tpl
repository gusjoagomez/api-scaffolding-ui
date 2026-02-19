version: "1.0"
method: PUT
path: "/{{.SubsystemLower}}/{{.TableNameLower}}/:id/update"
description: "Actualizar {{.EntityName}}"

auth:
  required: true
  permissions: ["{{.TableNameLower}}.write"]

params:
  path:
    - name: id
      type: {{range .PrimaryKeys}}{{if eq . "id"}}int{{else}}string{{end}}{{break}}{{else}}int{{end}}
      required: true
      validation:
        min: 1
      error_message: "Debe proporcionar un ID válido"
  
  body:
    {{- $hasPassword := false }}
    {{- range .Fields}}
    {{- if shouldIncludeInUpdate .Name}}
  
    - name: "{{.NameSnake}}"
      type: "{{.Type}}"
      required: false
      {{- if .Validation}}
      validation:
        {{- range $key, $value := .Validation}}
        {{- if (eq $key "pattern")}}
        {{$key}}: '{{printf "%v" $value}}'
        {{- else}}
        {{$key}}: {{printf "%v" $value}}
        {{- end}}
        {{- end}}
      {{- end}}
      {{- if or (eq .Name "password") (eq .Name "password_hash")}}
      constraints: [has_lower, has_upper, has_number, has_special]
      {{- $hasPassword = true }}
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
        {{- if shouldIncludeInUpdate .Name}}
      {{ print "{{if ." .Name "}}" }}{{.Name}} = :{{.NameSnake}},{{ print "{{end}}" }}
          {{- $first = false}}
        {{- end}}
      {{- end}}
      {{- if .HasAuditFields}}
      updated_at = {{nowFunc}}
      {{- end}}
      WHERE {{range .PrimaryKeys}}{{.}} = :id{{break}}{{else}}id = :id{{end}}
      RETURNING {{$first = true}}
      {{- range .Fields}}
        {{- if and (ne .Name "password_hash") (ne .Name "password") (ne .Name "created_at") (ne .Name "deleted_at") (ne .Name "deleted_by")}}
          {{- if shouldIncludeInUpdate .Name}}
            {{- if not $first}}, {{end}}{{.Name}}
            {{- $first = false}}
          {{- end}}
        {{- end}}
      {{- end}}
      {{- if $hasPassword}}
    transform_params:
      password_hash: "hash_password(:password)"
      {{- end}}
response:
  success:
    code: 200
    message: "{{.EntityName}} actualizado exitosamente"

hooks:
  after:
    - type: cache_invalidate
      keys: ["{{.TableNameLower}}:list:*", "{{.TableNameLower}}:{id}"]
    - type: notification
      event: "{{.TableNameLower}}.actualizado"