package event

type DashboardEventReason string

const (
	// DashboardSuccessed represent dashboard successed running
	DashboardSuccessed DashboardEventReason = "Successed"

	// DashboardApplied represent all resources applied
	DashboardApplied DashboardEventReason = "Applied"

	// DashboardReady represent health check passed
	DashboardReady DashboardEventReason = "Ready"
)
