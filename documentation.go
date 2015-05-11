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

func writeDoc(templ string, data interface{}, output io.Writer) error {
	tpl := template.Must(template.New("doc").Parse(templ))
	tabwriter := tabwriter.NewWriter(output, 0, 8, 1, ' ', 0)
	return tpl.Execute(tabwriter, data)
}

// Creates file at correct path. caller must close file.
func createDoc(name string) (*os.File, error) {
	tplName, err := filepath.Abs(
		filepath.Join(
			docPath,
			fmt.Sprintf("%s.md", strings.ToLower(name))))
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
	write("wercker", appTpl, app)

	cmdTpl, err := loadTemplate("subcmd")
	if err != nil {
		return err
	}

	for _, cmd := range app.Commands {
		write(cmd.Name, cmdTpl, cmd)
	}
	return nil
}
