# {{.Name}}

NAME
----
{{.Name}} - {{.Usage}}

USAGE
-----
command `{{.Name}}{{if .Flags}} [command options]{{end}} [arguments...]`{{if .Description}}

DESCRIPTION
-----------
{{.Description}}{{end}}{{if .Flags}}

OPTIONS
-------

{{range $flag := stringifyFlags $.Flags}}{{Prefixed $flag.Name}}::
  {{$flag.Usage}}{{if $flag.Value}}
  Default;;
    {{$flag.Value}}{{end}}
{{end}}{{end}}
