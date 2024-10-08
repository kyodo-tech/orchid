package persistence

import (
	"context"
	"errors"
	"time"
)

// WorkflowLogEntry represents a single append-only log entry for a workflow.
type WorkflowLogEntry struct {
	WorkflowID    string
	WorkflowName  string
	ActivityName  string
	ActivityToken string
	ActivityState TaskState
	NodeID        int64
	Input         []byte
	Output        []byte
	Config        *string
	Error         *string
	Timestamp     time.Time
	Duration      time.Duration
}

type TaskState string

const (
	StateOpen      TaskState = "open"
	StateCompleted TaskState = "completed"
	StateFailed    TaskState = "failed"
	StateTimedOut  TaskState = "timed_out"
)

// WorkflowStatus represents an append-only log entry for a workflow.
type WorkflowStatus struct {
	WorkflowID    string
	WorkflowName  string
	WorkflowState TaskState
	Timestamp     time.Time

	// NonRestorable indicates that the workflow is not recoverable on reboot.
	// By default, workflows are restorable.
	NonRestorable bool `json:"non_restorable"`

	// LeaseExpiresAt time.Time
	// LeaseHolderID  string
}

// type WorkflowOrchestrator interface {
// 	// AcquireOrRenewLease attempts to acquire a new lease for a workflow or renew an existing one.
// 	// Returns true if the lease was successfully acquired or renewed.
// 	AcquireOrRenewLease(ctx context.Context, workflowID string, holderInstanceID string, leaseDuration time.Duration) (bool, error)
//
// 	// ReleaseLease releases the lease for a given workflow, making it available for other instances.
// 	ReleaseLease(ctx context.Context, workflowID string, holderInstanceID string) error
// }

/*
func (o *Orchestrator) manageWorkflowLease(ctx context.Context, workflowID string) {
    leaseDuration := 1 * time.Minute // Example lease duration
    ticker := time.NewTicker(leaseDuration / 2) // Renew lease halfway through its duration
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            // Context cancelled, release lease and stop goroutine
            o.ReleaseLease(ctx, workflowID, o.instanceID)
            return
        case <-ticker.C:
            // Attempt to renew lease
            success, err := o.AcquireOrRenewLease(ctx, workflowID, o.instanceID, leaseDuration)
            if err != nil || !success {
                // Handle failed lease renewal: log error, attempt to release lease, and stop goroutine
                o.logError("Failed to renew lease", "workflowID", workflowID, "error", err)
                o.ReleaseLease(ctx, workflowID, o.instanceID)
                return
            }
        }
    }
}
*/

var ErrWorkflowIDExists = errors.New("execution ID already exists")

type Persister interface {
	IsUniqueWorkflowID(ctx context.Context, workflowID string) error

	LogWorkflowStep(ctx context.Context, entry *WorkflowLogEntry) error
	LogWorkflowStatus(ctx context.Context, workflowID string, status WorkflowStatus) error

	// LoadOpenWorkflows returns all workflows in the given state.
	// On boot, the orchestrator will call this method with state=StateOpen to
	// recover all open workflows.
	LoadOpenWorkflows(ctx context.Context, workflowName string) ([]*WorkflowStatus, error)

	// LoadWorkflowLog returns the last log entry for the given workflowID.
	// This is used to recover the state of an actitivy.
	// LatestWorkflowStepByWorkflowID(ctx context.Context, workflowID string) (*WorkflowLogEntry, error)

	// LoadWorkflowSteps returns all log entries for the given workflowID.
	LoadWorkflowSteps(ctx context.Context, workflowID string) ([]*WorkflowLogEntry, error)
}
