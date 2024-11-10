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
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type SQLitePersister struct {
	DB *sql.DB
}

// Ensure SQLitePersister implements the Persister interface.
var _ Persister = &SQLitePersister{}

func NewSQLitePersister(dbPath string) (*SQLitePersister, error) {
	if dbPath == ":memory:" {
		dbPath = "file::memory:?cache=shared"
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS workflow_execution_log (
		log_id INTEGER PRIMARY KEY,
		workflow_id TEXT NOT NULL,
		node_id INTEGER NOT NULL,
		activity_name TEXT NOT NULL,
		activity_token TEXT NOT NULL,
		state TEXT NOT NULL,
		input BLOB,
		output BLOB,
		config TEXT, -- JSON encoded configuration
		error TEXT, -- JSON encoded error if any
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		duration INTEGER NOT NULL -- Duration in milliseconds
	)`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS workflow_status_log (
		status_id INTEGER PRIMARY KEY,
		workflow_id TEXT NOT NULL,
		workflow_name TEXT,
		state TEXT NOT NULL,
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		parent_workflow_id TEXT,
		non_restorable BOOLEAN DEFAULT FALSE
	)`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_workflow_execution_log_workflow_id ON workflow_execution_log(workflow_id)`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_workflow_execution_log_workflow_id_timestamp ON workflow_execution_log(workflow_id, timestamp DESC)`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_workflow_status_log_workflow_id ON workflow_status_log(workflow_id)`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_workflow_status_log_state ON workflow_status_log(state)`)
	if err != nil {
		return nil, err
	}

	return &SQLitePersister{DB: db}, nil
}

func (s *SQLitePersister) IsUniqueWorkflowID(ctx context.Context, workflowID string) error {
	var count int
	query := `SELECT COUNT(*) FROM workflow_status_log WHERE workflow_id = ?`
	err := s.DB.QueryRowContext(ctx, query, workflowID).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrWorkflowIDExists
	}
	return nil
}

func (s *SQLitePersister) LogWorkflowStep(ctx context.Context, entry *WorkflowLogEntry) error {
	query := `INSERT INTO workflow_execution_log (workflow_id, node_id, activity_name, activity_token, state, input, output, config, error, timestamp, duration) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.DB.ExecContext(ctx, query, entry.WorkflowID, entry.NodeID, entry.ActivityName, entry.ActivityToken, entry.ActivityState, entry.Input, entry.Output, entry.Config, entry.Error, entry.Timestamp, entry.Duration.Milliseconds())
	return err
}

func (s *SQLitePersister) LogWorkflowStatus(ctx context.Context, status WorkflowStatus) error {
	query := `INSERT INTO workflow_status_log (workflow_id, workflow_name, state, timestamp, non_restorable, parent_workflow_id) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := s.DB.ExecContext(ctx, query, status.WorkflowID, status.WorkflowName, status.WorkflowState, status.Timestamp, status.NonRestorable, status.ParentWorkflowID)
	return err
}

// LoadWorkflows loads all workflows in a given state but have not reached a terminal state.
func (s *SQLitePersister) LoadOpenWorkflows(ctx context.Context, workflowName string) ([]*WorkflowStatus, error) {
	query := `
    SELECT wsl.workflow_id, wsl.state, wsl.timestamp, wsl.non_restorable, wsl.parent_workflow_id
    FROM workflow_status_log wsl
    WHERE (wsl.state = 'open' OR wsl.state = 'panicked')
	AND wsl.workflow_name = ?
    AND NOT EXISTS (
        SELECT 1
        FROM workflow_status_log
        WHERE workflow_id = wsl.workflow_id
        AND state IN ('failed', 'timed_out', 'completed')
    )
    `
	rows, err := s.DB.QueryContext(ctx, query, workflowName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statuses []*WorkflowStatus
	for rows.Next() {
		var ws WorkflowStatus
		err := rows.Scan(&ws.WorkflowID, &ws.WorkflowState, &ws.Timestamp, &ws.NonRestorable, &ws.ParentWorkflowID)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, &ws)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return statuses, nil
}

// Adjusted LoadWorkflowSteps method in SQLitePersister
func (s *SQLitePersister) LoadWorkflowSteps(ctx context.Context, workflowID string) ([]*WorkflowLogEntry, error) {
	query := `SELECT node_id, activity_name, input, output, error, state FROM workflow_execution_log WHERE workflow_id = ? ORDER BY timestamp ASC`
	rows, err := s.DB.QueryContext(ctx, query, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []*WorkflowLogEntry
	for rows.Next() {
		var step WorkflowLogEntry
		var input, output []byte
		if err := rows.Scan(&step.NodeID, &step.ActivityName, &input, &output, &step.Error, &step.ActivityState); err != nil {
			return nil, err
		}
		step.Input = input
		step.Output = output
		steps = append(steps, &step)
	}
	return steps, nil
}
