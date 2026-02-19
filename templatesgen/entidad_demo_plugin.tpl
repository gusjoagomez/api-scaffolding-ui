version: "1.0"
method: POST
path: "/{{.SubsystemLower}}/{{.TableNameLower}}/demo-plugin"
description: "Test endpoint for demo plugin - updates {{.TableNameLower}}"

auth:
  required: false

commands:
  - type: custom_function
    db: {{.DBName | default "main"}}
    function: demo  # Maps to demo.so plugin

response:
  success:
    code: 200
    message: "Demo plugin executed successfully"