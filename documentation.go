package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/codegangsta/cli"
)

const docPath = "Documentation/command"

// Stringifies the flag and returns the first line.
func shortFlag(flag cli.Flag) string {
	ss := strings.Split(flag.String(), "\n")
	if len(ss) == 0 {
		return ""
	}
	return ss[0]
}

// setupUsageFormatter configures codegangsta.cli to output usage
// information in the format we want.
func setupUsageFormatter(app *cli.App) {
	cli.CommandHelpTemplate = `NAME:
   {{.Name}} - {{.Usage}}

USAGE:
   command {{.Name}}{{if .Flags}} [command options]{{end}} [arguments...]{{if .Description}}

DESCRIPTION:
   {{.Description}}{{end}}{{if .Flags}}

OPTIONS:
{{range .Flags}}{{if not .IsHidden}}   {{. | shortFlag}}{{ "\n" }}{{end}}{{end}}{{end}}
`

	cli.HelpPrinter = func(templ string, data interface{}) {
		w := tabwriter.NewWriter(app.Writer, 0, 8, 1, '\t', 0)
		t := template.Must(template.New("help").Funcs(
			template.FuncMap{"shortFlag": shortFlag},
		).Parse(templ))
		err := t.Execute(w, data)
		if err != nil {
			panic(err)
		}
		w.Flush()
	}
}

func loadTemplate(templ string) (string, error) {
	absPath, err := filepath.Abs(filepath.Join(docPath, fmt.Sprintf("%s.tpl", templ)))
	if err != nil {
		return "", err
	}
	tpl, err := ioutil.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	return string(tpl), nil
}

func prefixFor(name string) (prefix string) {
	if len(name) == 1 {
		prefix = "-"
	} else {
		prefix = "--"
	}

	return
}

func prefixedNames(fullName string) (prefixed string) {
	parts := strings.Split(fullName, ",")
	for i, name := range parts {
		name = strings.Trim(name, " ")
		prefixed += prefixFor(name) + name
		if i < len(parts)-1 {
			prefixed += ", "
		}
	}
	return
}

// stringifyFlags gives us a representation of flags that's usable in templates
func stringifyFlags(flags []cli.Flag) ([]cli.StringFlag, error) {
	usefulFlags := []cli.StringFlag{}
	for _, flag := range flags {
		switch t := flag.(type) {
		default:
			return nil, fmt.Errorf("unexpected type %T", t)
		case cli.StringSliceFlag:
			usefulFlags = append(usefulFlags, cli.StringFlag{
				Name:  t.Name,
				Usage: t.Usage,
				Value: strings.Join(*t.Value, ","),
			})
		case cli.BoolFlag:
			usefulFlags = append(usefulFlags, cli.StringFlag{
				Name:  t.Name,
				Usage: t.Usage,
			})
		case cli.StringFlag:
			usefulFlags = append(usefulFlags, cli.StringFlag{
				Name:  t.Name,
				Usage: t.Usage,
				Value: t.Value,
			})
		case cli.IntFlag:
			usefulFlags = append(usefulFlags, cli.StringFlag{
				Name:  t.Name,
				Usage: t.Usage,
				Value: fmt.Sprintf("%d", t.Value),
			})
		case cli.Float64Flag:
			usefulFlags = append(usefulFlags, cli.StringFlag{
				Name:  t.Name,
				Usage: t.Usage,
				Value: fmt.Sprintf("%.2f", t.Value),
			})
		}
	}
	return usefulFlags, nil

}

func writeDoc(templ string, data interface{}, output io.Writer) error {
	funcMap := template.FuncMap{
		"stringifyFlags": stringifyFlags,
		"Prefixed":       prefixedNames,
	}
	tpl := template.Must(template.New("doc").Funcs(funcMap).Parse(templ))
	tabwriter := tabwriter.NewWriter(output, 0, 8, 1, ' ', 0)
	return tpl.Execute(tabwriter, data)
}

// Creates file at correct path. caller must close file.
func createDoc(name string) (*os.File, error) {
	tplName, err := filepath.Abs(
		filepath.Join(
			docPath,
			fmt.Sprintf("%s.adoc", strings.ToLower(name))))
	if err != nil {
		return nil, err
	}
	return os.Create(tplName)
}

// GenerateDocumentation generates docs for each command
func GenerateDocumentation(options *GlobalOptions, app *cli.App) error {

	write := func(name string, templ string, data interface{}) error {
		var w io.Writer = app.Writer

		if !options.Debug {
			doc, err := createDoc(name)
			if err != nil {
				return err
			}
			defer doc.Close()
			w = doc
		}
		return writeDoc(templ, data, w)
	}
	appTpl, err := loadTemplate("wercker")
	if err != nil {
		return err
	}
	if err := write("wercker", appTpl, app); err != nil {
		return err
	}

	cmdTpl, err := loadTemplate("subcmd")
	if err != nil {
		return err
	}

	for _, cmd := range app.Commands {
		if err := write(cmd.Name, cmdTpl, cmd); err != nil {
			return err
		}
	}
	return nil
}
