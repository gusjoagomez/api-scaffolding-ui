version: "1.0"
method: GET
path: "/{{.SubsystemLower}}/{{.TableNameLower}}/list"
description: "Lista de {{.EntityName}}"

auth:
  required: true
  permissions: ["{{.TableNameLower}}.read"]

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

commands:
  - type: query
    sql: |
      SELECT * 
      FROM {{.TableName}} 
      {{- if .HasSoftDelete}}
      WHERE activo = true 
      {{- end}}
      {{- if .HasSearch}}
      AND ({{range .SearchFields}}{{.}} ILIKE '%' || COALESCE(:search, ''){{break}}{{else}}id{{end}})
      {{- end}}
      ORDER BY {{range .PrimaryKeys}}{{.}}{{break}}{{else}}id{{end}} ASC LIMIT :limit OFFSET :offset
    #AND ( campo1 ILIKE '%' || COALESCE(:search, '') || '%'
    #      OR campo2 ILIKE '%' || COALESCE(:search, '') || '%' 
    #      OR campo3 ILIKE '%' || COALESCE(:search, '') || '%'
    #)
    returns: "multiple"
    transform_params:
      offset: "(:page - 1) * :limit"

response:
  success:
    code: 200
    message: "Registros obtenidos exitosamente"

  structure:
    type: paginated
    pagination:
      total_query: "SELECT COUNT(*) FROM {{.TableName}} WHERE activo = true"
      # AND ( nombre ILIKE '%' || COALESCE(:search, '') || '%'
      #      OR apellido ILIKE '%' || COALESCE(:search, '') || '%' 
      #      OR username ILIKE '%' || COALESCE(:search, '') || '%'
      #)

  success_code: 200

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
  ttl: 60
  key: "{{.TableNameLower}}:list:{{ "{{" }}.page{{ "}}" }}:{{ "{{" }}.limit{{ "}}" }}:{{ "{{" }}.search{{ "}}" }}"

