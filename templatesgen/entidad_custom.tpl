version: "1.0"
method: PUT
path: "/{{.SubsystemLower}}/{{.TableNameLower}}/:id/custom"
description: "Funcion custom para actualizar {{.EntityName}}"

auth:
  required: true
  permissions: ["{{.TableNameLower}}.write"]

params:
  path:
    - name: id
      type: int
      required: true
      validation:
        min: 1
      error_message: "Debe proporcionar un ID válido"
  
  body:
    # TODO: Definir parámetros específicos del negocio
    # - name: "campo"
    #   type: "tipo"
    #   required: true

commands:
  # TODO: Definir SQL o función personalizada
  - type: exec
    db: main
    sql: |
      # my_custom_function(:param1, :param2)

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