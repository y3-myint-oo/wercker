package main

import (
  // "sync"
  // "time"
  "bytes"
  "fmt"
  // "io/ioutil"
  "log"
  "os"
  // "code.google.com/p/go.net/websocket"
  "github.com/codegangsta/cli"
  "github.com/fsouza/go-dockerclient"
  "github.com/termie/go-shutil"
  // "gopkg.in/yaml.v1"
)


type ChanWriter struct {
  out chan string
}

func (cw *ChanWriter) Write(p []byte) (n int, err error) {
  var buf bytes.Buffer
  n, err = buf.Write(p)
  fmt.Println("written to", buf.String())
  cw.out <- buf.String();
  fmt.Println("afterchan")
  return n, err
}

func main() {
  app := cli.NewApp()
  app.Flags = []cli.Flag {
    cli.StringFlag{"projectDir", "./projects", "path where projects live"},
    cli.StringFlag{"stepDir", "./steps", "path where steps live"},
    cli.StringFlag{"buildDir", "./builds", "path where builds live"},

    cli.StringFlag{"dockerEndpoint", "tcp://127.0.0.1:4243", "docker api endpoint"},
    cli.StringFlag{"mntRoot", "/mnt", "directory on the guest where volumes are mounted"},
    cli.StringFlag{"guestRoot", "/pipeline", "directory on the guest where work is done"},
    cli.StringFlag{"buildId", "", "build id"},

    // These options might be overwritten by the wercker.yml
    cli.StringFlag{"sourceDir", "", "source path relative to checkout root"},
    cli.IntFlag{"noResponseTimeout", 5, "timeout if no script output is received in this many minutes"},
    cli.IntFlag{"commandTimeout", 10, "timeout if command does not complete in this many minutes"},
  }

  app.Commands = []cli.Command{
    {
      Name: "build",
      ShortName: "b",
      Usage: "build a project",
      Action: func(c *cli.Context) {
          println("building project: ", c.Args().First())
          BuildProject(c)
      },
      Flags: []cli.Flag {
      },
    },
    {
      Name: "run",
      ShortName: "r",
      Usage: "run some arbitrary stuff",
      Action: func(c *cli.Context) {
          // println("building project: ", c.Args().First())
          RunArbitrary(c)
      },
    },
    {
      Name: "parse",
      Usage: "parse the wercker.yml",
      Action: ParseYaml,
    },
  }
  app.Run(os.Args)
}


func BuildProject(c *cli.Context) {
  // endpoint := "tcp://127.0.0.1:4243"
  // client, _ := docker.NewClient(endpoint)

  // Parse CLI and local env
  options, err := CreateGlobalOptions(c, os.Environ())
  if err != nil {
    panic(err)
  }
  fmt.Println(options)

  // The project to build is the first arg
  // NOTE(termie): For now we are expecting it to be downloaded
  // before we start so we are just expecting it to exist in our
  // projects directory.
  project := c.Args().First()
  projectDir := fmt.Sprintf("%s/%s", options.ProjectDir, project)

  // Return a []byte of the yaml we find or create.
  werckerYaml, err := ReadYaml([]string{projectDir}, false)
  if err != nil {
    panic(err)
  }

  // Parse that bad boy.
  rawConfig, err := ConfigFromYaml(werckerYaml)
  if err != nil {
    panic(err)
  }

  // Promote the RawBuild to a real Build. We believe in you, Build!
  build, err := rawConfig.RawBuild.ToBuild(options)
  if err != nil {
    panic(err)
  }

  // Promote RawBox to a real Box. We believe in you, Box!
  box, err := rawConfig.RawBox.ToBox(build, options)
  if err != nil {
    panic(err)
  }

  fmt.Println("BOX", box.Name)
  fmt.Println("RAW STEPS", rawConfig.RawBuild.RawSteps)
  fmt.Println("STEPS", build.Steps)

  // Make sure we have the box available
  image, err := box.Fetch()
  if err != nil {
    panic(err)
  }

  fmt.Println("IMAGE", image.ID)


  // TODO(termie): Services go here

  // Start setting up the build dir
  err = os.MkdirAll(build.HostPath(), 0755)
  if err != nil {
    panic(err)
  }
  fmt.Println(projectDir)
  fmt.Println(build.HostPath("source"))
  err = shutil.CopyTree(projectDir, build.HostPath("source"), nil)
  if err != nil {
    panic(err)
  }

  // Make sure we have the steps
  for _, step := range build.Steps {
    path, err := step.Fetch()
    if err != nil {
      panic(err)
    }
    fmt.Println("STEP PATH", path)
  }

  // Make our list of binds for the Docker attach

}




func RunArbitrary(c *cli.Context) {
  endpoint := "tcp://127.0.0.1:4243"
  client, _ := docker.NewClient(endpoint)

  // // Import an image
  // err := client.PullImage(docker.PullImageOptions{Repository: "base"},
  //                         docker.AuthConfiguration{})

  // Delete the old container?
  err := client.RemoveContainer(
    docker.RemoveContainerOptions{ID: "one-off",
                                  Force: true})

  // Create a container for our command
  testContainer, err := client.CreateContainer(
    docker.CreateContainerOptions{
      Name: "one-off",
      Config: &docker.Config{
        Image: "base",
        Tty: false,
        OpenStdin: true,
        AttachStdin: true,
        AttachStdout: true,
        AttachStderr: true,
        Cmd: []string{"/bin/sh", "-c", c.Args().First()}}})

  if err != nil {
    log.Fatalln(err)
  }
  fmt.Println("Container ID: ", testContainer.ID)

  err = client.StartContainer(testContainer.ID, nil)
  if err != nil {
    log.Fatalln(err)
  }

  // wsUrl := fmt.Sprintf(
  //   "ws://127.0.0.1:4243/containers/%s/attach/ws?stdin=1&stderr=1&stdout=1&stream=1", testContainer.ID)

  // ws, err := websocket.Dial(wsUrl, "", "http://localhost/")
  // if err != nil {
  //   log.Fatalln(err)
  // }

  sess := CreateSession(endpoint, testContainer.ID)
  sess, err = sess.Attach()
  if err != nil {
    log.Fatalln(err)
  }

  // for {
  //   sess.Send([]string{"date"})
  //   fmt.Println(<-sess.ch)
  // }
  exitCode, recv, err := sess.SendChecked([]string{"date", "sleep 1", "date", "sleep 1", "date"})
  fmt.Println("exit code: ", exitCode)
  for i := range recv {
    fmt.Print(recv[i])
  }

  // var stderr bytes.Buffer
  // var listener = make(chan string, 2)
  // var stdout = ChanWriter{out:listener}
  // var stderr = ChanWriter{out:listener}

  // // success := make(chan struct{})
  // opts := docker.AttachToContainerOptions{
  //   Container: testContainer.ID,
  //   OutputStream: &stdout,
  //   ErrorStream: &stderr,
  //   Stream: true,
  //   Stdout: true,
  //   Stderr: true,
  //   // RawTerminal: true,
  //   // Logs: true,
  // }



  // go client.AttachToContainer(opts)
  // // if err != nil {
  // //   log.Fatal(err)
  // // }

  // var wg sync.WaitGroup
  // wg.Add(1)
  // go func () {
  //   fmt.Println("halala")
  //   for s := range listener {
  //     fmt.Println("Gotcha: ", s);

  //   }
  //   wg.Done()
  // }()

  // wg.Wait()
  // // success <- <-success
  // // v := <-success
  // // fmt.Println(v)
  // // fmt.Srintln(stdout.String())
  // // go func () {
  // //   time.Sleep(5 * time.Second)
  // //   fmt.Println(stdout.Len())
  // //   time.Sleep(5 * time.Second)
  // //   fmt.Println(stdout.Len())
  // //   wg.Done()
  // // }()

  // // wg.Wait()
}


func ParseYaml(c *cli.Context) {
  // config, err := ConfigFromYaml("projects/termie/farmboy/wercker.yml")
  // if err != nil {
  //   panic(err)
  // }
  // fmt.Println("CONFIG", config.Box)

  // file, err := ioutil.ReadFile("projects/termie/farmboy/wercker.yml")
  // if err != nil {
  //   log.Fatalln(err)
  // }

  // m := make(map[interface{}]interface{})

  // err = yaml.Unmarshal(file, &m)

  // build := m["build"].(map[interface{}]interface{})
  // steps := build["steps"].([]interface{})

  // for _, v := range steps {
  //   var stepId string
  //   stepData := make(map[string]string)

  //   // There is only one key in this array but can't just pop in golang
  //   for id, data := range v.(map[interface{}]interface{}) {
  //     stepId = id.(string)
  //     for prop, value := range data.(map[interface{}]interface{}) {
  //       stepData[prop.(string)] = value.(string)
  //     }
  //   }
  //   fmt.Println(stepId, stepData)
  // }



  // for k, v := range m {
  //   fmt.Printf("k: ", k, "v: ", v, "\n")
  // }

}
