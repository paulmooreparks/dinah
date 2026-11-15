//go:build !org_chart_acceptance

package bus

func (b *Bus) teamLeadAcceptanceCheckpoint() func() error { return nil }
