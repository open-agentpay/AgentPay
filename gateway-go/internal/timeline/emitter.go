package timeline

import (
	"context"

	"github.com/open-agentpay/agentpay-gateway-go/internal/agent"
)

// TimelineAuditEmitter wraps a timeline.Service as an agent.AuditEmitter.
type TimelineAuditEmitter struct {
	timelineSvc Service
	underlying  agent.AuditEmitter
}

// NewTimelineAuditEmitter creates an emitter that feeds both the timeline read model and an optional underlying emitter.
func NewTimelineAuditEmitter(timelineSvc Service, underlying agent.AuditEmitter) *TimelineAuditEmitter {
	return &TimelineAuditEmitter{
		timelineSvc: timelineSvc,
		underlying:  underlying,
	}
}

// Emit records the audit event into the timeline service and forwards to any underlying emitter.
func (e *TimelineAuditEmitter) Emit(event *agent.AuditEvent) {
	if event == nil {
		return
	}
	if e.timelineSvc != nil {
		_ = e.timelineSvc.RecordEvent(context.Background(), event)
	}
	if e.underlying != nil {
		e.underlying.Emit(event)
	}
}
