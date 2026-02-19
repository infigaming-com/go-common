package snowflake

// MetricsHook allows services to bridge snowflake metrics to their observability stack
// (e.g., OTel, Prometheus) without adding a direct dependency.
type MetricsHook interface {
	OnIDGenerated(count int)
	OnClockRollback()
	OnSequenceOverflow()
	OnLeaseAcquired(nodeID int64)
	OnLeaseRenewed()
	OnLeaseRenewFail()
	OnLeaseExpired()
	OnLeaseReleased()
}

// noopMetrics is the default no-op implementation.
type noopMetrics struct{}

func (noopMetrics) OnIDGenerated(int)     {}
func (noopMetrics) OnClockRollback()      {}
func (noopMetrics) OnSequenceOverflow()   {}
func (noopMetrics) OnLeaseAcquired(int64) {}
func (noopMetrics) OnLeaseRenewed()       {}
func (noopMetrics) OnLeaseRenewFail()     {}
func (noopMetrics) OnLeaseExpired()       {}
func (noopMetrics) OnLeaseReleased()      {}
