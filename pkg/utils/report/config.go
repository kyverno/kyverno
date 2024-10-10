package report

type ReportingConfiguration interface {
	ValidateReportsEnabled() bool
	MutateReportsEnabled() bool
	MutateExistingReportsEnabled() bool
	ImageVerificationReportsEnabled() bool
	GenerateReportsEnabled() bool
}
