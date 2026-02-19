version: "1.0"
method: GET
path: "/{{.TableNameLower}}/report"
description: "Reporte de {{.TableNameLower}} activos"

auth:
  required: true
  permissions: ["{{.TableNameLower}}.read"]

params:
  query:
    - name: format
      type: string
      required: true
      validation:
        pattern: '^(pdf|xls|csv)$'
      error_message: "Formato inválido. Valores permitidos: pdf, xls, csv"

    - name: fecha_desde
      type: string
      required: false
      validation:
        pattern: '^\d{4}-\d{2}-\d{2}$'
      error_message: "Formato de fecha inválido (YYYY-MM-DD)"

    - name: fecha_hasta
      type: string
      required: false
      validation:
        pattern: '^\d{4}-\d{2}-\d{2}$'
      error_message: "Formato de fecha inválido (YYYY-MM-DD)"

    {{- range .ReportFilterParams}}
    - name: {{.Name}}
      type: {{.Type}}
      required: {{.IsRequired}}
      {{- if .Default}}
      default: "{{.Default}}"
      {{- end}}
    {{- end}}

    - name: download
      type: string
      required: false
      default: "false"

report:
  name: "reporte_{{.TableNameLower}}_activos"
  title: "Listado de {{.EntityNameTitle}} Activos"
  description: "Reporte de todos los {{.TableNameLower}} activos del sistema"

  template:
    jrxml: "reports/templates/base_report.jrxml"
    params:
      REPORT_TITLE: "Listado de {{.EntityNameTitle}} Activos"
      REPORT_SUBTITLE: "Generado automáticamente"
      COMPANY_LOGO: "reports/assets/logo.png"
    page:
      size: "A4"
      orientation: "landscape"
      margins:
        top: 20
        bottom: 20
        left: 15
        right: 15

  data:
    db: {{.DBName | default "main"}}
    sql: |
      SELECT
        {{- $first := true}}
        {{- range .ReportFields}}
        {{- if $first}}{{$first = false}}{{else}},{{end}}
        {{.SQLExpr}}
        {{- end}}
      FROM {{.TableName}} {{.TableAlias | default (printf "%s" .TableNameLower)}}
      {{- range .ReportJoins}}
      {{.}}
      {{- end}}
      WHERE {{.TableAlias | default .TableNameLower}}.activo = true
      {{"{{"}}if .fecha_desde{{"}}"}} AND {{.TableAlias | default .TableNameLower}}.created_at >= :fecha_desde {{"{{"}}end{{"}}"}}
      {{"{{"}}if .fecha_hasta{{"}}"}} AND {{.TableAlias | default .TableNameLower}}.created_at <= :fecha_hasta {{"{{"}}end{{"}}"}}
      {{- range .ReportFilterParams}}
      {{"{{"}}if .{{.Name}}{{"}}"}}  AND {{$.TableAlias | default $.TableNameLower}}.{{.Name}} = :{{.Name}}  {{"{{"}}end{{"}}"}}
      {{- end}}
      ORDER BY {{range .ReportSortFields}}{{.}} ASC{{break}}{{else}}{{.TableNameLower}}.id ASC{{end}}

  columns:
    {{- $order := 1}}
    {{- range .ReportFields}}
    - field: "{{.Field}}"
      title: "{{.Title}}"
      type: {{.Type}}
      width: {{.Width | default 100}}
      align: "{{.Align | default "left"}}"
      {{- if .Format}}
      format: "{{.Format}}"
      {{- end}}
      order: {{$order}}
      {{- $order = add $order 1}}
    {{- end}}

  {{- if .ReportGroups}}
  groups:
    {{- range .ReportGroups}}
    - field: "{{.Field}}"
      title: "{{.Title}}"
      order: {{.Order}}
      show_header: {{.ShowHeader | default true}}
      show_footer: {{.ShowFooter | default true}}
      page_break: {{.PageBreak | default false}}
      sort: "{{.Sort | default "asc"}}"
    {{- end}}
  {{- end}}

  sorting:
    {{- range .ReportSortFields}}
    - field: "{{.Field}}"
      direction: "{{.Direction | default "asc"}}"
    {{- end}}

  format_options:
    pdf:
      font_family: "Helvetica"
      font_size: 9
      header_font_size: 11
      header_bg_color: "#2C3E50"
      header_font_color: "#FFFFFF"
      alternating_row_color: "#F2F3F4"
      show_grid_lines: true
      show_row_numbers: false

    xls:
      sheet_name: "{{.EntityNameTitle}}"
      freeze_header: true
      auto_filter: true
      auto_width: true
      header_bold: true
      date_format: "dd/mm/yyyy"
      number_format: "#,##0.00"

    csv:
      delimiter: ";"
      encoding: "UTF-8"
      include_header: true
      quote_fields: true
      date_format: "yyyy-MM-dd"
      decimal_separator: "."

output:
  storage: local
  config:
    base_path: "./reports/generated"
    filename_template: ":name_:timestamp"
    subfolder_strategy: "by_date"
    subfolder_template: ":year/:month"
    cleanup:
      enabled: true
      max_age_hours: 72

commands:
  - type: report

response:
  success:
    code: 200
    message: "Reporte generado exitosamente"
  map:
    url: "file_url"
  error:
    code: 500
    message: "Error al generar el reporte"

hooks:
  after:
    - type: notification
      event: "reportes.generado"

audit:
  enabled: true
  log_params: true

cache:
  enabled: false

rate_limit:
  enabled: true
  max_attempts: 5
  window: "1m"
  key: "report:{{ "{{" }}.user_id{{ "}}" }}"