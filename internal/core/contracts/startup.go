package contracts

// StartupInfo contains only non-sensitive information that is safe to show
// when the application is ready to receive requests.
type StartupInfo struct {
	Address       string
	Port          int
	APIURL        string
	HealthURL     string
	ReadinessURL  string
	DatabaseReady bool
}

// StartupReporter is notified after the database is connected and the HTTP
// listener has successfully reserved its port.
type StartupReporter interface {
	ReportStartup(info StartupInfo)
}
