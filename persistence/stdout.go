package persistence

import (
	"context"
	"log/slog"
)

// SlogDebugger is not a real persister, but a debugger that logs to slog.
// It is used for debugging purposes only and shows the log entries through slog.
type SlogDebugger struct {
	logger *slog.Logger
}

var _ Persister = &SlogDebugger{}

func NewSlogDebugger(logger *slog.Logger) *SlogDebugger {
	return &SlogDebugger{logger: logger}
}

func (s *SlogDebugger) IsUniqueWorkflowID(ctx context.Context, workflowID string) error {
	return nil
}

func (s *SlogDebugger) LogWorkflowStep(ctx context.Context, entry *WorkflowLogEntry) error {
	s.logger.Info("WorkflowStep", "workflowID", entry.WorkflowID, "nodeID", entry.NodeID, "activityName", entry.ActivityName, "activityToken", entry.ActivityToken, "state", entry.ActivityState, "input", string(entry.Input), "output", string(entry.Output), "config", entry.Config, "error", entry.Error, "timestamp", entry.Timestamp, "duration", entry.Duration)
	return nil
}

func (s *SlogDebugger) LogWorkflowStatus(ctx context.Context, workflowID string, status WorkflowStatus) error {
	s.logger.Info("WorkflowStatus", "workflowID", status.WorkflowID, "state", status.WorkflowState, "timestamp", status.Timestamp, "nonRecoverable", status.NonRestorable)
	return nil
}

func (s *SlogDebugger) LoadOpenWorkflows(ctx context.Context, workflowName string) ([]*WorkflowStatus, error) {
	return nil, nil
}

//func (s *SlogDebugger) LatestWorkflowStepByWorkflowID(ctx context.Context, workflowID string) (*WorkflowLogEntry, error) {
//	return nil, nil
//}

func (s *SlogDebugger) LoadWorkflowSteps(ctx context.Context, workflowID string) ([]*WorkflowLogEntry, error) {
	return nil, nil
}