package main


type Build struct {
  steps []Step
  options *GlobalOptions
}


func (b *RawBuild) Build(options *GlobalOptions) (*Build, error) {
  return &Build{}, nil
}
