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

package orchid

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
)

type Snapshotter struct {
	mu            sync.Mutex
	data          map[string][]byte
	attributeData map[string]map[string]interface{}
	fields        []string
}

// NewShapshotter initializes a new checkpoint activity with data storage.
func NewSnapshotter() *Snapshotter {
	return &Snapshotter{
		data:          make(map[string][]byte),
		attributeData: make(map[string]map[string]interface{}),
	}
}

// NewAttributeShapshotter can be used for both full and partial snapshotting.
func NewAttributeShapshotter(fields []string) *Snapshotter {
	return &Snapshotter{
		data:          make(map[string][]byte),
		attributeData: make(map[string]map[string]interface{}),
		fields:        fields,
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

func (chp *Snapshotter) SaveAttributes(ctx context.Context, input []byte) ([]byte, error) {
	chp.mu.Lock()
	defer chp.mu.Unlock()

	workflowID := WorkflowID(ctx)

	// Parse input JSON
	var inputData map[string]interface{}
	if err := json.Unmarshal(input, &inputData); err != nil {
		return nil, err
	}

	// Check if we have cached fields for this workflow
	if cachedFields, ok := chp.attributeData[workflowID]; ok {
		// Inject cached fields into inputData
		for field, cachedValue := range cachedFields {
			setNestedField(inputData, field, cachedValue)
		}

		// Return modified JSON with injected fields
		output, err := json.Marshal(inputData)
		if err != nil {
			return nil, err
		}

		return output, nil
	}

	// If no cached fields, cache specified fields and return unmodified input

	// Extract specified fields
	cachedData := make(map[string]interface{})
	for _, field := range chp.fields {
		value := getNestedField(inputData, field)
		if value != nil {
			cachedData[field] = value
		}
	}

	// Save cached data
	chp.attributeData[workflowID] = cachedData

	return input, nil
}

func setNestedField(data map[string]interface{}, field string, value interface{}) {
	keys := strings.Split(field, ".")
	var current interface{} = data
	for i, key := range keys {
		if i == len(keys)-1 {
			if m, ok := current.(map[string]interface{}); ok {
				m[key] = value
			}
		} else {
			if m, ok := current.(map[string]interface{}); ok {
				if _, exists := m[key]; !exists {
					m[key] = make(map[string]interface{})
				}
				current = m[key]
			}
		}
	}
}

func getNestedField(data map[string]interface{}, field string) interface{} {
	keys := strings.Split(field, ".")
	var current interface{} = data
	for _, key := range keys {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[key]
		} else {
			return nil
		}
	}
	return current
}

// Delete removes checkpoint data for a given workflow ID.
func (chp *Snapshotter) Delete(ctx context.Context, input []byte) ([]byte, error) {
	chp.mu.Lock()
	defer chp.mu.Unlock()

	workflowID := WorkflowID(ctx)
	delete(chp.data, workflowID)
	delete(chp.attributeData, workflowID)

	return input, nil
}
