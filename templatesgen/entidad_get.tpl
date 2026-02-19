version: "1.0"
method: GET
path: "/{{.SubsystemLower}}/{{.TableNameLower}}/:id/get"
description: "Obtener {{.EntityName}} por ID"

auth:
  required: true
  permissions: ["{{.TableNameLower}}.read"]

params:
  path:
    - name: id
      type: {{range .PrimaryKeys}}{{if eq . "id"}}int{{else}}string{{end}}{{break}}{{else}}int{{end}}
      required: true
      validation:
        min: 1
      error_message: "Debe proporcionar ID válido"

commands:
  # Obtener {{.EntityNameTitle}}
  - type: query
    sql: |
      SELECT *
      FROM {{.TableName}} 
      WHERE {{range .PrimaryKeys}}{{.}} = :id{{break}}{{else}}id = :id{{end}}{{- if .HasSoftDelete}} AND activo = true {{- end}}
    returns: "single"
    on_result:
      if_not_found:
        action: "abort"
        message: "{{.EntityNameTitle}} no encontrado"
    ## Guardar empresa_id para usarlo en includes
    #transform_params:
    #  empresa_id: "empresa_id"

response:
  success:
    code: 200
    message: "{{.EntityNameTitle}} obtenido exitosamente"
  error:
    code: 404
    message: "{{.EntityNameTitle}} no encontrado"
  
  map:
    {{- range .Fields}}
    {{- if and (ne .Name "password_hash") (ne .Name "password") (ne .Name "created_at") (ne .Name "updated_at") (ne .Name "deleted_at") (ne .Name "deleted_by")}}
    {{.Name}}: "{{.NameSnake}}"
    {{- end}}
    {{- end}}
    {{- if .HasAuditFields}}
    created_at: "created_at"
    {{- end}}

  {{- if .Includes}}
  includes:
    {{- $first := true}}
    {{- range .Includes}}
    ####
    {{ if not $first}}#{{end}}- relation: {{.Relation}}
    {{ if not $first}}#{{end}}  query: |
    {{ if not $first}}#{{end}}    {{.Query | indent 8}}
    {{ if not $first}}#{{end}}  type: {{.Type}}
    {{- $first = false}}
    {{- end}}
  {{- end}}

cache:
  enabled: true
  ttl: 300
  key: "{{.TableNameLower}}:{{"{{"}}.id{{"}}"}}"

audit:
  enabled: true