## {{.Name}}

### NAME:
   {{.Name}} - {{.Usage}}

### USAGE:
   command `{{.Name}}{{if .Flags}} [command options]{{end}} [arguments...]`{{if .Description}}

### DESCRIPTION:
   {{.Description}}{{end}}{{if .Flags}}

### OPTIONS:
```
{{range .Flags}}   {{.}}{{ "\n" }}{{end}}```{{end}}
