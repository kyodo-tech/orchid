// Copyright 2024 Kyodo Tech合同会社
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	StatePanicked  TaskState = "panicked" // retryable
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

	ParentWorkflowID *string
}

var ErrWorkflowIDExists = errors.New("workflow ID already exists")

type Persister interface {
	IsUniqueWorkflowID(ctx context.Context, workflowID string) error

	LogWorkflowStep(ctx context.Context, entry *WorkflowLogEntry) error
	LogWorkflowStatus(ctx context.Context, status WorkflowStatus) error

	// LoadOpenWorkflows returns all workflows in the given state.
	// On boot, the orchestrator will call this method with state=StateOpen to
	// recover all open workflows.
	LoadOpenWorkflows(ctx context.Context, workflowName string) ([]*WorkflowStatus, error)

	// LoadWorkflowSteps returns all log entries for the given workflowID.
	LoadWorkflowSteps(ctx context.Context, workflowID string) ([]*WorkflowLogEntry, error)
}
