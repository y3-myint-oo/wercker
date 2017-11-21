package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/codegangsta/cli"
	"github.com/davecgh/go-spew/spew"
	"github.com/docker/docker/builder/dockerfile/instructions"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/wercker/wercker/core"
	"github.com/wercker/wercker/util"
)

var dockerfileCommand = cli.Command{
	Name:  "dockerfile",
	Usage: "dockerfile a project",
	Action: func(c *cli.Context) {
		envfile := c.GlobalString("environment")
		env := util.NewEnvironment(os.Environ()...)
		env.LoadFile(envfile)

		// settings := util.NewCLISettings(c)
		// opts, err := core.NewBuildOptions(settings, env)
		// if err != nil {
		//   cliLogger.Errorln("Invalid options\n", err)
		//   os.Exit(1)
		// }
		// dockerOptions, err := dockerlocal.NewOptions(settings, env)
		// if err != nil {
		//   cliLogger.Errorln("Invalid options\n", err)
		//   os.Exit(1)
		// }
		err := cmdDockerfile(context.Background())
		if err != nil {
			cliLogger.Fatal(err)
		}
	},
	Flags: FlagsFor(WerckerInternalFlagSet),
}

func cmdDockerfile(ctx context.Context) error {
	// logger := util.RootLogger().WithField("Logger", "Main")

	file, err := os.Open("Dockerfile")
	if err != nil {
		return err
	}

	res, err := parser.Parse(file)
	// spew.Dump(res)

	ast := res.AST
	stages, metaArgs, err := instructions.Parse(ast)
	if err != nil {
		return err
	}
	spew.Dump(stages)
	spew.Dump(metaArgs)

	df := Dockerfile{}
	for i, stage := range stages {
		name := stage.Name
		if name == "" {
			name = fmt.Sprintf("stage-%d", i)
		}
		df.StartPipeline(name)
		df.HandleBox(stage.BaseName)
		// First do a pass to load any meta info we might need
		for _, cmd := range stage.Commands {
			df.HandleMeta(cmd)
		}
		// looking at meta generate any directories, env vars, etc
		df.GeneratePreSteps()
		for _, cmd := range stage.Commands {
			df.HandleSteps(cmd)
		}
		// looking at meta check whether we need to push, copy
		// things to output
		df.GeneratePostSteps()
	}

	return nil
}

type Dockerfile struct {
	Pipelines map[string]*core.PipelineConfig
	current   *core.PipelineConfig
	// Meta holds on to some variables we'll need to track
	meta *DockerMeta
}

type DockerMeta struct {
	workdir    string
	entrypoint string
}

func (d *Dockerfile) StartPipeline(name string) {
	if d.Pipelines == nil {
		d.Pipelines = map[string]*core.PipelineConfig{}
	}
	d.current = &core.PipelineConfig{}
	d.meta = &DockerMeta{}
	d.Pipelines[name] = d.current
	fmt.Printf("%s:\n", name)
}

func (d *Dockerfile) HandleBox(box string) {
	d.current.Box = &core.RawBoxConfig{}
	d.current.Box.BoxConfig = &core.BoxConfig{ID: box}
	fmt.Printf("  box: %s\n", box)
}

func (d *Dockerfile) HandleMeta(cmd instructions.Command) {
	// A variety of meta info commands to store before we
	// start making steps
	switch v := cmd.(type) {
	case *instructions.WorkdirCommand:
		fmt.Printf("  workdir=%s\n", v.Path)
		d.meta.workdir = v.Path
	case *instructions.EntrypointCommand:
		fmt.Printf("  entrypoint=%s\n", v.String())
		d.meta.entrypoint = v.String()
	default:
		fmt.Printf("  Unhandled Command: %s\n", cmd.Name())
	}
}

func (d *Dockerfile) HandleSteps(cmd instructions.Command) {
	// pass
}

func (d *Dockerfile) GeneratePreSteps() {
	// pass
}

func (d *Dockerfile) GeneratePostSteps() {
	// pass
}

// func flattenNodes(o []string, node *parser.Node) []string {
//   if node.Value != "" {
//     o = append(o, node.Value)
//   }
//   if node.Next != nil {
//     return flatteNodes(o, node.Next)
//   }
//   return o
// }

// func (d *Dockerfile) Handle(node *parser.Node) {
//   switch cmd := node.value; cmd {
//   case "from":
//     // We're expecting these as the beginnings of pipelines
//     d.HandleFROM(node)
//   }
// }

// func (d *Dockerfile) HandleFROM(node *parser.Node) error {
//   if node.Next == nil {
//     return fmt.Errorf("no from in from")
//   }
//   parts := flattenNodes([]string{}, node.Next)
// }
