package orchid

import (
	"context"
	"sync"
)

// Snapshotter with cleanup capability
type Snapshotter struct {
	mu   sync.Mutex
	data map[string][]byte
}

// NewShapshotter initializes a new checkpoint activity with data storage.
func NewShapshotter() *Snapshotter {
	return &Snapshotter{
		data: make(map[string][]byte),
	}
}

// Save stores data on first invocation, returns stored data on subsequent calls.
func (chp *Snapshotter) Save(ctx context.Context, input []byte) ([]byte, error) {
	chp.mu.Lock()
	defer chp.mu.Unlock()

	workflowID := WorkflowID(ctx)
	if existingData, ok := chp.data[workflowID]; ok {
		// Return saved data on subsequent invocations
		return existingData, nil
	}

	// Save data on the first invocation
	chp.data[workflowID] = input
	return input, nil
}

// delete removes checkpoint data for the specified workflow ID.
func (chp *Snapshotter) delete(workflowID string) {
	chp.mu.Lock()
	defer chp.mu.Unlock()
	delete(chp.data, workflowID)
}

// Delete removes checkpoint data for a given workflow ID.
func (chp *Snapshotter) Delete(ctx context.Context, input []byte) ([]byte, error) {
	workflowID := WorkflowID(ctx)
	chp.delete(workflowID)
	return input, nil
}
