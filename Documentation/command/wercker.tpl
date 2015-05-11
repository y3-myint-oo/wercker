## {{.Name}}

### Name:
   {{.Name}} - {{.Usage}}

### USAGE:
   {{.Name}} {{if .Flags}}[global options] {{end}}command{{if .Flags}} [command options]{{end}} [arguments...]

### VERSION:
   {{.Version}}{{if or .Author .Email}}

### AUTHOR:{{if .Author}}
  {{.Author}}{{if .Email}} - <{{.Email}}>{{end}}{{else}}
  {{.Email}}{{end}}{{end}}

### COMMANDS:
   {{range .Commands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
   {{end}}{{if .Flags}}
### GLOBAL OPTIONS:
```
{{range .Flags}}   {{.}}{{ "\n" }}{{end}}```{{end}}
