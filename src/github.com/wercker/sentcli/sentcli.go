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
  app.Commands = []cli.Command{
    {
      Name: "build",
      ShortName: "b",
      Usage: "build a project",
      Action: func(c *cli.Context) {
          println("building project: ", c.Args().First())
          BuildProject(c)
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
  exitCode, recv, err := sess.SendChecked([]string{"date", "date", "date"})
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


func BuildProject(c *cli.Context) {
  endpoint := "tcp://127.0.0.1:4243"
  client, _ := docker.NewClient(endpoint)
  // imgs, _ := client.ListImages(true)
  // for _, img := range imgs {
  //   fmt.Println("ID: ", img.ID)
  //   fmt.Println("RepoTags: ", img.RepoTags)
  //   fmt.Println("Tag: ", img.Tag)
  //   fmt.Println("Repository: ", img.Repository)
  // }

  conts, _ := client.ListContainers(docker.ListContainersOptions{All: true})
  for _, cont := range conts {
    fmt.Println("ID: ", cont.ID)
    fmt.Println("Image: ", cont.Image)
    fmt.Println("Names: ", cont.Names)
    fmt.Println("Command: ", cont.Command)
    fmt.Println("Status: ", cont.Status)
    // fmt.Println("Tag: ", cont.Tag)
    // fmt.Println("Repository: ", cont.Repository)
  }

  // var imageId string = "b750fe79269d2ec9a3c593ef05b4332b1d1a02a62b4accb2c21d589ff2f5f2dc"

  // testContainer, err := client.CreateContainer(
  //   docker.CreateContainerOptions{
  //     Name: "foo-test6",
  //     Config: &docker.Config{
  //       Image: imageId,
  //       AttachStdout: true,
  //       AttachStderr: true,
  //       Cmd: []string{"/bin/echo", "Hello World"}}})

  // if err != nil {
  //   log.Fatalln(err)
  // }
  // fmt.Println("Container ID: ", testContainer.ID)

  // var containerId string = "4063c8f4a5c8de342015f6f1ab8462fcd5972f1dd69a2e7ffaaa3ef5f33bb45f"
  // var containerId string = "5a8fbd92d10cc3fba7b1802c47016ae28b084ab2411dbb84c54394eb723ff775"
  var containerId string = "efabaf3e5f5c83f25b29fcb72f5d3e7fc502a324db65f96c3cb1dc21dda166b8"


  var stdout, stderr bytes.Buffer
  opts := docker.AttachToContainerOptions{
    Container: containerId,
    OutputStream: &stdout,
    ErrorStream: &stderr,
    // Stream: true,
    Stdout: true,
    Stderr: true,
    Logs: true,
  }

  err := client.AttachToContainer(opts)
  if err != nil {
    log.Fatal(err)
  }

  err = client.StartContainer(containerId, nil)
  if err != nil {
    log.Fatalln(err)
  }

  fmt.Println(stdout.String())
  // fmt.Println(stderr.String())

  // version, _ := client.Version()
  // fmt.Println("Version: ", version.Get("Version"))

  // client.PullImage(docker.PullImageOptions{Repository: "base"},
  //                  docker.AuthConfiguration{Username:""})
  println("picture me buildin.")
}


type MapStringString []map[string]string


func ParseYaml(c *cli.Context) {
  config, err := ConfigFromYaml("projects/termie/farmboy/wercker.yml")
  if err != nil {
    panic(err)
  }
  fmt.Println("CONFIG", config.Box)

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
