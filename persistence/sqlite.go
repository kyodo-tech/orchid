package persistence

import (
	"context"
	"database/sql"
	"encoding/json"

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
	config, _ := json.Marshal(entry.Config)
	errorStr, _ := json.Marshal(entry.Error)
	query := `INSERT INTO workflow_execution_log (workflow_id, node_id, activity_name, activity_token, state, input, output, config, error, timestamp, duration) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.DB.ExecContext(ctx, query, entry.WorkflowID, entry.NodeID, entry.ActivityName, entry.ActivityToken, entry.ActivityState, entry.Input, entry.Output, string(config), string(errorStr), entry.Timestamp, entry.Duration.Milliseconds())
	return err
}

func (s *SQLitePersister) LogWorkflowStatus(ctx context.Context, workflowID string, status WorkflowStatus) error {
	query := `INSERT INTO workflow_status_log (workflow_id, workflow_name, state, timestamp, non_restorable) VALUES (?, ?, ?, ?, ?)`
	_, err := s.DB.ExecContext(ctx, query, workflowID, status.WorkflowName, status.WorkflowState, status.Timestamp, status.NonRestorable)
	return err
}

// LoadWorkflows loads all workflows in a given state but have not reached a terminal state.
func (s *SQLitePersister) LoadOpenWorkflows(ctx context.Context, workflowName string) ([]*WorkflowStatus, error) {
	query := `
    SELECT wsl.workflow_id, wsl.state, wsl.timestamp, wsl.non_restorable
    FROM workflow_status_log wsl
    WHERE wsl.state = 'open'
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
		err := rows.Scan(&ws.WorkflowID, &ws.WorkflowState, &ws.Timestamp, &ws.NonRestorable)
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

// LatestWorkflowStepByWorkflowID returns the last log entry for the given workflowID.
// func (s *SQLitePersister) LatestWorkflowStepByWorkflowID(ctx context.Context, workflowID string) (*WorkflowLogEntry, error) {
// 	query := `SELECT node_id, activity_name, activity_token, state, input, output, config, error, timestamp, duration FROM workflow_execution_log WHERE workflow_id = ? ORDER BY log_id DESC LIMIT 1`
// 	var entry WorkflowLogEntry
// 	var configStr, errorStr string
// 	err := s.DB.QueryRowContext(ctx, query, workflowID).Scan(&entry.NodeID, &entry.ActivityName, &entry.ActivityToken, &entry.ActivityState, &entry.Input, &entry.Output, &configStr, &errorStr, &entry.Timestamp, &entry.Duration)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if configStr != "" {
// 		json.Unmarshal([]byte(configStr), &entry.Config)
// 	}
// 	if errorStr != "" {
// 		json.Unmarshal([]byte(errorStr), &entry.Error)
// 	}
// 	return &entry, nil
// }

// Adjusted LoadWorkflowSteps method in SQLitePersister
func (s *SQLitePersister) LoadWorkflowSteps(ctx context.Context, workflowID string) ([]*WorkflowLogEntry, error) {
	query := `SELECT node_id, activity_name, input, output, state FROM workflow_execution_log WHERE workflow_id = ? ORDER BY timestamp ASC`
	rows, err := s.DB.QueryContext(ctx, query, workflowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []*WorkflowLogEntry
	for rows.Next() {
		var step WorkflowLogEntry
		var input, output []byte
		if err := rows.Scan(&step.NodeID, &step.ActivityName, &input, &output, &step.ActivityState); err != nil {
			return nil, err
		}
		step.Input = input
		step.Output = output
		steps = append(steps, &step)
	}
	return steps, nil
}
