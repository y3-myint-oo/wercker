package main


type Report struct {
  JobId string
}

type ReportFunc func(Report) error


type Reporter struct {
  reportFunc ReportFunc
  options *GlobalOptions
}


func CreateReporter(options *GlobalOptions) *Reporter {
  return &Reporter{options:options}
}


func (r *Reporter) SetReportFunc(reportFunc ReportFunc) {
  r.reportFunc = reportFunc
}


func (r *Reporter) Report (report Report) error {
  return r.reportFunc(report)
}
