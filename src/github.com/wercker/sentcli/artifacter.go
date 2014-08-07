package main

type Artifact struct {

}

type Artifacter struct {
  options *GlobalOptions
}


func CreateArtifacter(options *GlobalOptions) *Artifacter {
  return &Artifacter{options:options}
}


func (a *Artifacter) Store(bucket, path string, artifact *Artifact) error {
  return nil
}
