package main


type Artifacter struct {
  options *GlobalOptions
}


func CreateArtifacter(options *GlobalOptions) *Artifacter {
  return &Artifacter{options:options}
}


func (a *Artifacter) Store(
