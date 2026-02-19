version: "1.0"
method: POST
path: "/{{.TableNameLower}}/:id/upload-avatar"
description: "Subir archivo de avatar del {{.EntityName}} al servidor local"

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

  file:
    - name: archivo
      type: file
      required: true
      validation:
        max_size: {{.UploadMaxSize | default 5242880}}
        allowed_types: {{.UploadAllowedTypes | default `["image/jpeg", "image/png", "image/webp"]`}}
        allowed_extensions: {{.UploadAllowedExtensions | default `[".jpg", ".jpeg", ".png", ".webp"]`}}
      error_message: "Archivo inválido. Solo imágenes JPG, PNG o WebP hasta 5MB"

commands:
  # Verificar que el {{.EntityName}} existe
  - type: validation
    db: {{.DBName | default "main"}}
    sql: "SELECT COUNT(*) as count FROM {{.TableName}} WHERE {{range .PrimaryKeys}}{{.}} = :id{{break}}{{else}}id = :id{{end}}{{if .HasSoftDelete}} AND activo = true{{end}}"
    condition: "count = 0"
    on_true:
      action: stop
      http_code: 404
      message: "{{.EntityNameTitle}} no encontrado"

  # Guardar archivo en disco local
  - type: file_save
    storage: local
    config:
      base_path: "./uploads/{{.TableNameLower}}/avatars"
      filename_strategy: "uuid"
      subfolder_strategy: "by_id"
      subfolder_template: ":id"
      overwrite: false
      create_dirs: true

  # Actualizar referencia en base de datos
  - type: exec
    db: {{.DBName | default "main"}}
    sql: |
      UPDATE {{.TableName}} SET
        avatar_path = :file_path,
        avatar_url = :file_url,
        updated_at = NOW(),
        updated_by = :user_id
        WHERE {{range .PrimaryKeys}}{{.}} = :id{{break}}{{else}}id = :id{{end}}
      RETURNING {{range .PrimaryKeys}}{{.}}{{break}}{{else}}id{{end}}, username, nombre, apellido, avatar_url, updated_at

response:
  success:
    code: 200
    message: "Avatar subido exitosamente"
  error:
    code: 400
    message: "Error al subir archivo"

map:
  {{range .PrimaryKeys}}{{.}}: "{{.}}"
  {{break}}{{else}}id: "id"
  {{end}}username: "username"
  nombre: "nombre"
  apellido: "apellido"
  avatar_url: "avatar_url"
  updated_at: "updated_at"

hooks:
  after:
    - type: cache_invalidate
      keys: ["{{.TableNameLower}}:list:*", "{{.TableNameLower}}:{{ "{{" }}.id{{ "}}" }}"]
    - type: notification
      event: "{{.TableNameLower}}.avatar_actualizado"

audit:
  enabled: true
  log_params: true
  sensitive_fields: []
  exclude_params: ["archivo"]

security:
  require_https: true

rate_limit:
  enabled: true
  max_attempts: 10
  window: "1m"
  key: "upload:{{ "{{" }}.user_id{{ "}}" }}"