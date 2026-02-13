version: "1.0"
method: DELETE
path: "/{{.TableNameLower}}/:id/delete"
description: "{{if .HasSoftDelete}}Desactivar{{else}}Eliminar{{end}} {{.EntityName}} (soft delete)"

auth:
  required: true
  permissions: ["{{.TableNameLower}}.delete"]

params:
  path:
    - name: id
      type: int
      required: true
      validation:
        min: 1
      error_message: "Debe proporcionar ID válido a {{if .HasSoftDelete}}desactivar{{else}}desactivar{{end}}"

commands:
  # Verificar que el registro existe
  - type: validation
    sql: "SELECT COUNT(*) as count FROM {{.TableName}} WHERE id = :id"
    condition: "count = 0"
    on_true:
      action: stop
      http_code: 404
      message: "{{.EntityNameTitle}} no encontrado"

  {{if .HasActiveField}}
  # Verificar si ya esta inactivo
  - type: validation
    sql: "SELECT COUNT(*) as count FROM {{.TableName}} WHERE id = :id AND activo = true"
    condition: "count = 0"
    on_true:
      action: stop
      http_code: 404
      message: "{{.EntityNameTitle}} no encontrado o ya está inactivo"
  {{end}}

  {{if .HasSoftDelete}}
  # Soft delete (marcar como inactivo o con deleted_at)
  - type: exec
    sql: |
      UPDATE {{.TableName}} SET {{if .HasActiveField}}activo = false,{{end}}deleted_at = {{if eq .DBDriver "postgres"}}NOW(){{else}}NOW(){{end}} {{if .HasDeletedBy}}, deleted_by = :user_id{{end}} WHERE id = :id
  {{else if .HasActiveField}}
  # Marcar como inactivo si existe campo activo
  - type: exec
    sql: |
      UPDATE {{.TableName}} SET activo = false WHERE id = :id
  {{else}}
  # Hard delete (eliminar permanentemente)
  - type: exec
    sql: |
      DELETE FROM {{.TableName}} WHERE id = :id
  {{end}}

response:
  success:
    code: 200
    message: "{{.EntityNameTitle}} {{if .HasSoftDelete}}desactivado{{else}}eliminado{{end}} exitosamente"

hooks:
  after:
    - type: cache_invalidate
      keys: ["{{.TableNameLower}}:list:*", "{{.TableNameLower}}:{{"{{."}}id{{"}}"}}"]
    - type: notification
      event: "{{.TableNameLower}}.{{if .HasSoftDelete}}desactivado{{else}}eliminado{{end}}"

audit:
  enabled: true