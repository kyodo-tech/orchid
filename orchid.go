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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"

	"github.com/kyodo-tech/orchid/persistence"
)

type NodeType string

const (
	Trigger NodeType = "trigger"
	Action  NodeType = "action"
)

var (
	ErrNodeNotFound                 = fmt.Errorf("node not found")
	ErrWorkflowNodeAlreadyExists    = fmt.Errorf("node already exists")
	ErrWorkflowInvalid              = fmt.Errorf("invalid, it has no trigger or no starting node")
	ErrWorkflowHasNoTriggers        = fmt.Errorf("no triggers found")
	ErrWorkflowHasNoNodes           = fmt.Errorf("workflow has no nodes")
	ErrOrchestratorActivityNotFound = fmt.Errorf("activity not found")
	ErrOrchestratorHasNoPersister   = fmt.Errorf("no persister set")
	ErrNoWorkflowID                 = fmt.Errorf("no execution ID set")
)

type Workflow struct {
	Name  string           `json:"name"`
	Nodes map[string]*Node `json:"nodes"`
	Edges []*Edge          `json:"edges"`

	activity2Node map[string]graph.Node `json:"-"`
	node2Activity map[int64]string      `json:"-"`
	directedGraph *simple.DirectedGraph `json:"-"`
}

func NewWorkflow(name string) *Workflow {
	if name == "" {
		name = "orchid"
	}

	return &Workflow{
		Name:          name,
		Nodes:         make(map[string]*Node),
		Edges:         make([]*Edge, 0),
		activity2Node: make(map[string]graph.Node),
		node2Activity: make(map[int64]string),
		directedGraph: simple.NewDirectedGraph(),
	}
}

// getNodeByActivityName returns the Node associated with an activity.
func (wf *Workflow) getNodeByActivityName(activity string) (*Node, bool) {
	node, exists := wf.Nodes[activity]
	return node, exists
}

// getActivityNameByNodeID returns the activity name associated with a node ID.
func (wf *Workflow) getActivityNameByNodeID(id int64) (string, bool) {
	activity, exists := wf.node2Activity[id]
	return activity, exists
}

func (wf *Workflow) AddNewNode(activity string, options ...NodeOption) *Workflow {
	node := NewNode(activity, options...)
	return wf.AddNode(node)
}

func (wf *Workflow) ThenNewNode(activity string, options ...NodeOption) *Workflow {
	node := NewNode(activity, options...)
	return wf.Then(node)
}

// AddNode adds a new task to the workflow.
func (wf *Workflow) AddNode(node *Node) *Workflow {
	if _, exists := wf.Nodes[node.ActivityName]; exists {
		panic(fmt.Sprintf("node %s: %v", node.ActivityName, ErrWorkflowNodeAlreadyExists))
	}

	var n graph.Node
	if node.ID < 0 {
		n = wf.directedGraph.NewNode()
	} else {
		n = simple.Node(node.ID)
	}

	wf.directedGraph.AddNode(n)
	node.ID = n.ID()

	wf.activity2Node[node.ActivityName] = n
	wf.node2Activity[node.ID] = node.ActivityName
	wf.Nodes[node.ActivityName] = node

	return wf
}

func (wf *Workflow) Then(node *Node) *Workflow {
	lastNode := wf.getLastNode()
	wf.AddNode(node)

	if len(wf.Nodes) == 0 {
		return wf
	}

	// Link the last added node to the new node
	wf.Link(lastNode.ActivityName, node.ActivityName)
	return wf
}

func (wf *Workflow) getLastNode() *Node {
	var lastNode *Node
	maxID := int64(-1)
	for _, node := range wf.Nodes {
		if node.ID > maxID {
			maxID = node.ID
			lastNode = node
		}
	}
	return lastNode
}

func (wf *Workflow) AddNodes(nodes ...*Node) *Workflow {
	for _, node := range nodes {
		wf.AddNode(node)
	}
	return wf
}

func (wf *Workflow) Link(from, to string) *Workflow {
	wf.AddEdge(&Edge{From: from, To: to})
	return wf
}

func (wf *Workflow) AddEdge(edge *Edge) error {
	var from, to graph.Node
	var exists bool
	if from, exists = wf.activity2Node[edge.From]; !exists {
		return fmt.Errorf("node %s: %w", edge.From, ErrNodeNotFound)
	}
	if to, exists = wf.activity2Node[edge.To]; !exists {
		return fmt.Errorf("node %s: %w", edge.To, ErrNodeNotFound)
	}

	wf.Edges = append(wf.Edges, edge)
	wf.directedGraph.SetEdge(wf.directedGraph.NewEdge(from, to))

	return nil
}

func (wf *Workflow) Export() ([]byte, error) {
	data, err := json.Marshal(wf)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal workflow: %w", err)
	}

	return data, nil
}

func (wf *Workflow) Import(workflow []byte) error {
	tmp := NewWorkflow("")
	if err := json.Unmarshal(workflow, tmp); err != nil {
		return err
	}

	for _, node := range tmp.Nodes {
		wf.AddNode(node)
	}

	for _, edge := range tmp.Edges {
		if err := wf.AddEdge(edge); err != nil {
			return err
		}
	}

	return nil
}

// markParallelNodes identifies nodes that are part of parallel paths between a fan-out and a fan-in node.
func markParallelNodes(g *simple.DirectedGraph) map[int64]struct{} {
	parallel := make(map[int64]struct{})
	visited := make(map[int64]struct{})
	recursionStack := make([]int64, 0)

	// Start DFS traversal from root nodes (nodes with no incoming edges)
	roots := findRootNodes(g)
	for _, root := range roots {
		dfsMarkParallelNodes(g, root, visited, recursionStack, parallel)
	}
	return parallel
}

// dfsMarkParallelNodes performs a DFS traversal to identify and mark parallel nodes.
func dfsMarkParallelNodes(g *simple.DirectedGraph, node graph.Node, visited map[int64]struct{}, recursionStack []int64, parallel map[int64]struct{}) {
	nodeID := node.ID()

	// If already visited, return.
	if _, ok := visited[nodeID]; ok {
		return
	}
	visited[nodeID] = struct{}{}
	recursionStack = append(recursionStack, nodeID) // Add to recursion stack

	// Identify fan-out nodes with outgoing edges to non-ancestors.
	successors := g.From(nodeID)
	nonAncestorSuccessors := []graph.Node{}
	for successors.Next() {
		successor := successors.Node()
		successorID := successor.ID()

		if !contains(recursionStack, successorID) {
			nonAncestorSuccessors = append(nonAncestorSuccessors, successor)
		}
	}

	if len(nonAncestorSuccessors) > 1 {
		// Node is a fan-out node. Start separate traversals for each non-ancestor successor.
		for _, successor := range nonAncestorSuccessors {
			traversePath(g, successor, map[int64]struct{}{}, parallel, nodeID, make(map[int64]bool), recursionStack)
		}
	} else {
		// Continue DFS traversal.
		for _, successor := range nonAncestorSuccessors {
			dfsMarkParallelNodes(g, successor, visited, recursionStack, parallel)
		}
		// Also need to consider back edges for standard traversal.
		successors.Reset()
		for successors.Next() {
			successor := successors.Node()
			successorID := successor.ID()
			if contains(recursionStack, successorID) {
				// Back edge to ancestor; continue traversal without marking parallel.
				dfsMarkParallelNodes(g, successor, visited, recursionStack, parallel)
			}
		}
	}

	// Remove from recursion stack
	// recursionStack = recursionStack[:len(recursionStack)-1]
}

// traversePath performs traversal from a fan-out node's successor to identify parallel nodes.
func traversePath(g *simple.DirectedGraph, node graph.Node, visited map[int64]struct{}, parallel map[int64]struct{}, startNodeID int64, recursionStack map[int64]bool, ancestors []int64) {
	nodeID := node.ID()

	// Stop traversal if we loop back to the starting node or an ancestor.
	if nodeID == startNodeID || contains(ancestors, nodeID) {
		return
	}

	// Detect cycles.
	if recursionStack[nodeID] {
		return
	}
	recursionStack[nodeID] = true
	defer func() { recursionStack[nodeID] = false }()

	// If the node has already been visited in this path, skip it.
	if _, ok := visited[nodeID]; ok {
		return
	}
	visited[nodeID] = struct{}{}

	// Check if the node is a fan-in node (has multiple predecessors from different paths).
	if isFanInNode(g, nodeID, ancestors) {
		// Node is a fan-in point; stop traversal.
		return
	}

	// Mark the node as parallel.
	parallel[nodeID] = struct{}{}

	// Continue traversal to successors.
	successors := g.From(nodeID)
	for successors.Next() {
		successor := successors.Node()
		traversePath(g, successor, visited, parallel, startNodeID, recursionStack, ancestors)
	}
}

// isFanInNode checks if a node has multiple predecessors not in the current path (ancestors).
func isFanInNode(g *simple.DirectedGraph, nodeID int64, ancestors []int64) bool {
	predecessors := g.To(nodeID)
	count := 0
	for predecessors.Next() {
		predID := predecessors.Node().ID()
		if !contains(ancestors, predID) {
			count++
			if count > 1 {
				return true
			}
		}
	}
	return false
}

// findRootNodes finds nodes with no incoming edges.
func findRootNodes(g *simple.DirectedGraph) []graph.Node {
	var roots []graph.Node
	nodes := g.Nodes()
	for nodes.Next() {
		node := nodes.Node()
		predecessors := g.To(node.ID())
		if predecessors.Len() == 0 {
			roots = append(roots, node)
		}
	}
	return roots
}

// Helper function to check if a slice contains an element.
func contains(slice []int64, elem int64) bool {
	for _, item := range slice {
		if item == elem {
			return true
		}
	}
	return false
}

func (wf *Workflow) startingGraphNodes() []graph.Node {
	var startNodes []graph.Node
	for nodes := wf.directedGraph.Nodes(); nodes.Next(); {
		node := nodes.Node()

		// if the node has no incoming edges, it is a starting node
		if wf.directedGraph.To(node.ID()).Len() == 0 {
			startNodes = append(startNodes, node)
		}
	}
	return startNodes
}

func (wf *Workflow) startingNodes() []*Node {
	var startNodes []*Node
	for _, node := range wf.Nodes {
		if wf.isStartNode(node) {
			startNodes = append(startNodes, node)
		}
	}
	return startNodes
}

func (wf *Workflow) exitNodes() []*Node {
	var exitNodes []*Node
	for _, node := range wf.Nodes {
		// If the node has no outgoing edges, it's an exit node
		if wf.directedGraph.From(node.ID).Len() == 0 {
			exitNodes = append(exitNodes, node)
		}
	}
	return exitNodes
}

func (wf *Workflow) startNode() (*Node, error) {
	var nodes []*Node

	startingNodes := wf.startingGraphNodes()

	for _, node := range startingNodes {
		nodeActivity, _ := wf.getActivityNameByNodeID(node.ID())
		if n, ok := wf.getNodeByActivityName(nodeActivity); ok {
			if n.IsTrigger() {
				// append the nodes the trigger is linked to
				links := wf.directedGraph.From(node.ID())
				for links.Next() {
					activity, _ := wf.getActivityNameByNodeID(links.Node().ID())
					if n, ok := wf.getNodeByActivityName(activity); ok {
						nodes = append(nodes, n)
					}
				}
			} else {
				nodes = append(nodes, n)
			}
		}
	}

	// either we must have triggers or we must have exactly one starting node
	if len(nodes) != 1 {
		return nil, fmt.Errorf("workflow %s: %w", wf.Name, ErrWorkflowInvalid)
	}

	return nodes[0], nil
}

type Node struct {
	ID           int64                  `json:"id"`
	ActivityName string                 `json:"activity"`
	Config       map[string]interface{} `json:"config,omitempty"`
	Type         NodeType               `json:"type,omitempty"`
	RetryPolicy  *RetryPolicy           `json:"retry,omitempty"`
	EditLink     *string                `json:"link,omitempty"`
}

type NodeOption func(*Node)

func WithNodeID(id int64) NodeOption {
	return func(n *Node) {
		n.ID = id
	}
}

func WithNodeType(t NodeType) NodeOption {
	return func(n *Node) {
		n.Type = t
	}
}

func WithNodeConfig(config map[string]interface{}) NodeOption {
	return func(n *Node) {
		n.Config = config
	}
}

func WithNodeRetryPolicy(policy *RetryPolicy) NodeOption {
	return func(n *Node) {
		n.RetryPolicy = policy
	}
}

func WithNodeEditLink(link string) NodeOption {
	return func(n *Node) {
		n.EditLink = &link
	}
}

func NewNode(activity string, options ...NodeOption) *Node {
	n := &Node{
		ID:           -1,
		ActivityName: activity,
		Type:         Action,
	}

	for _, option := range options {
		option(n)
	}

	return n
}

func (n *Node) IsTrigger() bool {
	return n.Type == Trigger
}

func (n *Node) RetryPolicyOrDefault(d *RetryPolicy) *RetryPolicy {
	if n.RetryPolicy == nil {
		if d == nil {
			return DefaultRetryPolicy()
		}

		return d
	}

	return n.RetryPolicy
}

func (n *Node) IsRetryableError(err error) bool {
	if n.RetryPolicy == nil {
		return false
	}

	if len(n.RetryPolicy.NonRetriableErrorReasons) == 0 {
		return true
	}

	// Unwrap DynamicRoute errors
	var routeErr *DynamicRoute
	if errors.As(err, &routeErr) {
		err = routeErr.Err
	}

	if err == nil {
		return true
	}

	for _, reason := range n.RetryPolicy.NonRetriableErrorReasons {
		if err.Error() == reason {
			return false
		}
	}

	return true
}

type RetryPolicy struct {
	MaxRetries               int
	InitInterval             time.Duration
	MaxInterval              time.Duration
	BackoffCoefficient       float64
	NonRetriableErrorReasons []string
}

// backoff calculates the exponential backoff time for a given attempt.
// The formula is:
//
//	time=(initial * coefficient) ^ (attempt−1)
func (p *RetryPolicy) backoff(attempt int) time.Duration {
	if attempt < 1 {
		return p.InitInterval
	}

	expBackoff := float64(p.InitInterval) * math.Pow(p.BackoffCoefficient, float64(attempt-1))
	if expBackoff > float64(p.MaxInterval) {
		return p.MaxInterval
	}
	return time.Duration(expBackoff)
}

// Default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:         0,
		InitInterval:       time.Second,
		MaxInterval:        10 * time.Minute,
		BackoffCoefficient: 2.0,
	}
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Activity func(ctx context.Context, input []byte) (output []byte, err error)

type Middleware func(Activity) Activity

type Reducer func([][]byte) []byte

type DynamicRoute struct {
	Key string `json:"key"`
	Err error  `json:"error"`
}

func (e *DynamicRoute) Error() string {
	return e.Err.Error()
}

func (e *DynamicRoute) Unwrap() error {
	return e.Err
}

func (e *DynamicRoute) MarshalJSON() ([]byte, error) {
	return json.Marshal(*e)
}

func (e *DynamicRoute) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, e)
}

func RouteTo(nodeKey string) error {
	return &DynamicRoute{Key: nodeKey}
}

type Orchestrator struct {
	workflow           *Workflow
	persister          persistence.Persister
	registry           map[string]Activity
	reducer            map[string]Reducer
	logger             *slog.Logger
	defaultRetryPolicy *RetryPolicy

	parallel            map[int64]struct{}
	completedNodeOutput map[int64][]byte
	completedNodeErrors map[int64]*DynamicRoute

	middlewares []Middleware

	failWorkflowOnActivityPanic bool
}

func NewOrchestrator(options ...OrchestratorOption) *Orchestrator {
	o := &Orchestrator{
		registry:            make(map[string]Activity),
		reducer:             make(map[string]Reducer),
		parallel:            make(map[int64]struct{}),
		completedNodeOutput: make(map[int64][]byte),
		completedNodeErrors: make(map[int64]*DynamicRoute),
		logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	for _, option := range options {
		option(o)
	}

	return o
}

type OrchestratorOption func(*Orchestrator)

func WithPersistence(persister persistence.Persister) OrchestratorOption {
	return func(o *Orchestrator) {
		o.persister = persister
	}
}

func WithLogger(logger *slog.Logger) OrchestratorOption {
	return func(o *Orchestrator) {
		o.logger = logger
	}
}

func WithDefaultRetryPolicy(policy *RetryPolicy) OrchestratorOption {
	return func(o *Orchestrator) {
		o.defaultRetryPolicy = policy
	}
}

func WithFailWorkflowOnActivityPanic(fail bool) OrchestratorOption {
	return func(o *Orchestrator) {
		o.failWorkflowOnActivityPanic = fail
	}
}

func (o *Orchestrator) LoadWorkflow(w *Workflow) {
	// check if activities are registered
	for _, node := range w.Nodes {
		if _, exists := o.registry[node.ActivityName]; !exists {
			panic(fmt.Sprintf("activity %s not registered", node.ActivityName))
		}
	}
	o.parallel = markParallelNodes(w.directedGraph)
	o.workflow = w
}

func (o *Orchestrator) RegisterActivity(name string, activity Activity) {
	wrappedActivity := activity
	for _, mw := range o.middlewares {
		wrappedActivity = mw(wrappedActivity)
	}
	o.registry[name] = wrappedActivity
}

func (o *Orchestrator) RegisterReducer(name string, reducer Reducer) {
	o.reducer[name] = reducer
}

func (o *Orchestrator) Use(mw Middleware) {
	o.middlewares = append(o.middlewares, mw)
}

func (o *Orchestrator) GetActivity(node *Node) (Activity, bool) {
	activity, exists := o.registry[node.ActivityName]
	return activity, exists
}

func (o *Orchestrator) GetReducer(node *Node) (Reducer, bool) {
	reducer, exists := o.reducer[node.ActivityName]
	return reducer, exists
}

func (o *Orchestrator) GetNode(graphNode graph.Node) (*Node, bool) {
	activity, exists := o.workflow.getActivityNameByNodeID(graphNode.ID())
	if !exists {
		return nil, false
	}

	node, ok := o.workflow.getNodeByActivityName(activity)
	return node, ok
}

type OrchestratorRegistry map[string]*Orchestrator

func NewOrchestratorRegistry() OrchestratorRegistry {
	return make(map[string]*Orchestrator)
}

func (r OrchestratorRegistry) Set(name string, o *Orchestrator) {
	r[name] = o
}

func (r OrchestratorRegistry) Get(name string) (*Orchestrator, error) {
	if o, ok := r[name]; ok {
		return o, nil
	}
	return nil, fmt.Errorf("orchestrator %s not found", name)
}

type configKey struct{}

func Config(ctx context.Context, key string) (interface{}, bool) {
	if v, ok := ctx.Value(configKey{}).(map[string]interface{}); ok {
		val, ok := v[key]
		return val, ok
	}

	return nil, false
}

type nameKey struct{}

func ActivityName(ctx context.Context) string {
	if v, ok := ctx.Value(nameKey{}).(string); ok {
		return v
	}

	return ""
}

func ConfigString(ctx context.Context, key string) (string, bool) {
	if v, ok := Config(ctx, key); ok {
		if val, ok := v.(string); ok {
			return val, ok
		}
	}

	return "", false
}

func ConfigInt(ctx context.Context, key string) (int, bool) {
	if v, ok := Config(ctx, key); ok {
		if val, ok := v.(int); ok {
			return val, ok
		}
	}

	return 0, false
}

type startKey struct{}

func SyncExecutor(ctx context.Context) (Activity, bool) {
	if v, ok := ctx.Value(startKey{}).(Activity); ok {
		return v, ok
	}

	return nil, false
}

type startAsyncKey struct{}

func AsyncExecutor(ctx context.Context) (Activity, bool) {
	if v, ok := ctx.Value(startAsyncKey{}).(Activity); ok {
		return v, ok
	}

	return nil, false
}

type loggerKey struct{}

func Logger(ctx context.Context) *slog.Logger {
	if v, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return v
	}

	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type activityTokenKey struct{}

func ActivityToken(ctx context.Context) string {
	if v, ok := ctx.Value(activityTokenKey{}).(string); ok {
		return v
	}

	return ""
}

func withActivityToken(ctx context.Context) context.Context {
	if ActivityToken(ctx) == "" {
		ctx = context.WithValue(ctx, activityTokenKey{}, uuid.New().String())
	}

	return ctx
}

type ctxKeyType string

const activityStartTimeKey ctxKeyType = "activity_start_time_"

func (n *Node) ActivityStartTime(ctx context.Context) time.Time {
	if v, ok := ctx.Value(activityStartTimeKey + ctxKeyType(n.ActivityName)).(time.Time); ok {
		return v
	}

	return time.Time{}
}

func (n *Node) withActivityStartTime(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, activityStartTimeKey+ctxKeyType(n.ActivityName), t)
}

func (o *Orchestrator) withNodeContext(ctx context.Context, node *Node) context.Context {
	ctx = context.WithValue(ctx, configKey{}, node.Config)
	ctx = context.WithValue(ctx, loggerKey{}, o.logger)
	ctx = context.WithValue(ctx, nameKey{}, node.ActivityName)

	return ctx
}

type workflowIDKey struct{}

func WithWorkflowID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, workflowIDKey{}, id)
}

func WithNewWorkflowID(ctx context.Context) context.Context {
	return context.WithValue(ctx, workflowIDKey{}, uuid.New().String())
}

func WorkflowID(ctx context.Context) string {
	v, _ := ctx.Value(workflowIDKey{}).(string)
	return v
}

type parentWorkflowIDKey struct{}

func WithParentWorkflowID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, parentWorkflowIDKey{}, id)
}

func ParentWorkflowID(ctx context.Context) string {
	v, _ := ctx.Value(parentWorkflowIDKey{}).(string)
	return v
}

type isRestoringMainWorkflow struct{}

func WithRestoringMainWorkflow(ctx context.Context) context.Context {
	return context.WithValue(ctx, isRestoringMainWorkflow{}, true)
}

func IsRestoringMainWorkflow(ctx context.Context) bool {
	v, _ := ctx.Value(isRestoringMainWorkflow{}).(bool)
	return v
}

type nonRestorableKey struct{}

func WithNonRestorable(ctx context.Context) context.Context {
	return context.WithValue(ctx, nonRestorableKey{}, true)
}

func IsNonRestorable(ctx context.Context) bool {
	v, _ := ctx.Value(nonRestorableKey{}).(bool)
	return v
}

func (o *Orchestrator) withTriggerContext(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, startKey{}, Activity(o.Start))
	ctx = context.WithValue(ctx, startAsyncKey{}, Activity(o.StartAsync))

	return ctx
}

func (o *Orchestrator) RunTriggers(ctx context.Context, input []byte) ([]byte, error) {
	var haveTrigger bool
	for _, node := range o.workflow.Nodes {
		if node.IsTrigger() {
			ctx1 := o.withNodeContext(ctx, node)
			ctx1 = o.withTriggerContext(ctx1)
			trigger, ok := o.GetActivity(node)
			if !ok {
				return nil, fmt.Errorf("trigger %s: %w", node.ActivityName, ErrOrchestratorActivityNotFound)
			}

			go trigger(ctx1, input)
			haveTrigger = true
		}
	}

	if !haveTrigger {
		return nil, ErrWorkflowHasNoTriggers
	}

	return nil, nil
}

func (o *Orchestrator) Start(ctx context.Context, data []byte) (output []byte, err error) {
	if o.workflow == nil {
		panic("no workflow loaded")
	}

	id := WorkflowID(ctx)
	if id == "" {
		return nil, fmt.Errorf("execution ID not set")
	}

	if o.persister != nil {
		err := o.persister.IsUniqueWorkflowID(ctx, id)
		if err != nil && !IsRestoringMainWorkflow(ctx) {
			if errors.Is(err, persistence.ErrWorkflowIDExists) {
				// Workflow already exists, attempt to restore
				restorable, err := o.RestorableWorkflows(ctx)
				if err != nil {
					return nil, err
				}
				if workflows, ok := restorable[o.workflow.Name]; ok {
					if restorable, ok := workflows[id]; ok {
						// Restore the workflow
						return restorable.Entrypoint(ctx)
					}
				}
				return nil, fmt.Errorf("workflow %s cannot be restored", id)
			}
			return nil, err
		}
	}

	startNode, err := o.workflow.startNode()
	if err != nil {
		return nil, err
	}

	o.tryUpdatePersistenceStatus(ctx, persistence.StateOpen)
	return o.executeNodeChain(ctx, startNode, data)
}

func (o *Orchestrator) StartAsync(ctx context.Context, data []byte) (output []byte, err error) {
	if o.workflow == nil {
		panic("no workflow loaded")
	}

	id := WorkflowID(ctx)
	if id == "" {
		return nil, fmt.Errorf("execution ID not set")
	}

	if o.persister != nil {
		err := o.persister.IsUniqueWorkflowID(ctx, id)
		if err != nil && !IsRestoringMainWorkflow(ctx) {
			if errors.Is(err, persistence.ErrWorkflowIDExists) {
				// Workflow already exists, attempt to restore
				restorable, err := o.RestorableWorkflows(ctx)
				if err != nil {
					return nil, err
				}
				if workflows, ok := restorable[o.workflow.Name]; ok {
					if restorable, ok := workflows[id]; ok {
						// Restore the workflow
						go func() {
							if _, err := restorable.Entrypoint(ctx); err != nil {
								Logger(ctx).Error("failed to restore workflow", "workflow", id, "error", err)
							}
						}()
						return nil, nil
					}
				}
				return nil, fmt.Errorf("workflow %s cannot be restored", id)
			}
			return nil, err
		}
	}

	startNode, err := o.workflow.startNode()
	if err != nil {
		return nil, err
	}

	o.tryUpdatePersistenceStatus(ctx, persistence.StateOpen)
	data, nodes, err := o.executeStep(ctx, startNode, data)
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return data, nil
	}
	startingNode := nodes[0]

	go func() {
		if _, err := o.executeNodeChain(ctx, startingNode, data); err != nil {
			Logger(ctx).Error("failed to execute node chain", "error", err)
		}
	}()

	return data, nil
}

func (o *Orchestrator) nextNodes(node *Node, err error) ([]*Node, error) {
	var route *DynamicRoute
	if err != nil && errors.As(err, &route) {
		nextNode, ok := o.workflow.getNodeByActivityName(route.Key)
		if !ok {
			return nil, fmt.Errorf("node %s: %w", route.Key, ErrNodeNotFound)
		}

		return []*Node{nextNode}, nil
	} else if err != nil {
		return nil, err
	}

	next := o.workflow.directedGraph.From(node.ID)
	if next.Len() == 0 {
		return nil, nil
	}

	var nextNodes []*Node
	for next.Next() {
		nextNode, ok := o.GetNode(next.Node())
		if !ok {
			return nil, fmt.Errorf("node %d: %w", next.Node().ID(), ErrNodeNotFound)
		}
		nextNodes = append(nextNodes, nextNode)
	}

	return nextNodes, nil
}

func isPanic(err error) bool {
	for err != nil {
		if errors.Is(err, ErrPanic) {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

func (o *Orchestrator) executeStep(ctx context.Context, node *Node, data []byte) ([]byte, []*Node, error) {
	select {
	case <-ctx.Done():
		Logger(ctx).Error("context done", "workflow", o.workflow.Name, "error", ctx.Err())
		o.tryUpdatePersistenceStatus(ctx, persistence.StateTimedOut)
		return nil, nil, ctx.Err()
	default:
	}

	var err error
	// Check if node has already completed
	prvData, ok := o.completedNodeOutput[node.ID]
	prvErr := o.completedNodeErrors[node.ID]
	if ok {
		data = prvData
		err = prvErr
	} else {
		data, err = o.executeNode(ctx, node, data)
		if isPanic(err) && !o.failWorkflowOnActivityPanic {
			// we cannot continue but the workflow is generally retryable after
			// a code fix. We can keep it open.
			return nil, nil, err
		}
	}

	nextNodes, err := o.nextNodes(node, err)
	if err != nil {
		// Check if the node has retries left
		if !node.hasRetriesLeft(ctx) {
			Logger(ctx).Error("failed to execute node", "workflow", o.workflow.Name, "node", node.ActivityName, "error", err)
			o.tryUpdatePersistenceStatus(ctx, persistence.StateFailed)
		}
		return nil, nil, err
	}

	if len(nextNodes) == 0 {
		o.tryUpdatePersistenceStatus(ctx, persistence.StateCompleted)
		return data, nil, nil
	}

	return data, nextNodes, nil
}

// retry key per activity
const retryAttemptsKey ctxKeyType = "retry_attempts_"

func (n *Node) hasRetriesLeft(ctx context.Context) bool {
	attempts, _ := ctx.Value(retryAttemptsKey + ctxKeyType(n.ActivityName)).(int)
	return attempts < n.RetryPolicyOrDefault(nil).MaxRetries
}

func (n *Node) withRetryAttempts(ctx context.Context, attempts int) context.Context {
	return context.WithValue(ctx, retryAttemptsKey+ctxKeyType(n.ActivityName), attempts)
}

func (n *Node) RetryAttempts(ctx context.Context) int {
	attempts, _ := ctx.Value(retryAttemptsKey + ctxKeyType(n.ActivityName)).(int)
	return attempts
}

// executeNodeChain executes a chain of nodes and returns the output.
// If the chain contains a merge point, the outputs are merged using the reducer
// if present or uses the preceding fan-out nodes output. Parallel nodes must
// converge at a merge node.
func (o *Orchestrator) executeNodeChain(ctx context.Context, node *Node, data []byte) ([]byte, error) {
	ctx = node.withActivityStartTime(ctx, time.Now().UTC())
	ctx = node.withRetryAttempts(ctx, 0)

	var err error
	var nextNodes []*Node
	for {
		attempts := node.RetryAttempts(ctx)
		ctx = node.withRetryAttempts(ctx, attempts+1)

		data, nextNodes, err = o.executeStep(ctx, node, data)
		if err != nil {
			return nil, err
		}

		if len(nextNodes) == 0 {
			return data, nil
		}

		if len(nextNodes) == 1 {
			node = nextNodes[0]
			continue
		}

		// Multiple next nodes, execute them in parallel
		for len(nextNodes) > 1 {
			var wg sync.WaitGroup
			errorsCh := make(chan error, len(nextNodes))
			outputs := make([][]byte, len(nextNodes))
			nextNodeSet := make(map[int64]*Node)
			nextNodeSetMutex := sync.Mutex{}

			for i, nextNode := range nextNodes {
				wg.Add(1)
				go func(i int, n *Node) {
					defer wg.Done()
					dataOut, nextNodesOut, err := o.executeStep(ctx, n, data)
					if err != nil {
						errorsCh <- err
						return
					}
					outputs[i] = dataOut

					// Collect next nodes
					nextNodeSetMutex.Lock()
					for _, nn := range nextNodesOut {
						nextNodeSet[nn.ID] = nn
					}
					nextNodeSetMutex.Unlock()
				}(i, nextNode)
			}

			wg.Wait()
			close(errorsCh)

			if len(errorsCh) > 0 {
				return nil, <-errorsCh
			}

			// Update nextNodes with the collected next nodes
			nextNodes = make([]*Node, 0, len(nextNodeSet))
			for _, n := range nextNodeSet {
				nextNodes = append(nextNodes, n)
			}

			// Merge point reached
			if len(nextNodes) == 1 {
				// Apply reducer if the next node is a merge point
				node = nextNodes[0]
				if reducer, ok := o.GetReducer(node); ok {
					data = reducer(outputs)
				} else {
					// Default behavior: use the first output or the data before parallelization
					data = outputs[0]
				}
				// continue with the merge point activity
				break
			}
		}
	}
}

func (o *Orchestrator) tryUpdatePersistenceStatus(ctx context.Context, state persistence.TaskState) {
	if o.persister == nil {
		return
	}

	id := WorkflowID(ctx)
	if id == "" {
		Logger(ctx).Error("persister present but no workflow ID set")
		return
	}

	p := DefaultRetryPolicy()
	p.MaxRetries = math.MaxInt

	for i := 0; ; i++ {
		if err := o.updatePersistenceStatus(ctx, id, state); err == nil {
			break
		}

		delay := p.backoff(i)
		Logger(ctx).Error("failed to update persistence status", "attempt", i, "delay", delay)
		<-time.After(delay)
	}
}

func (o *Orchestrator) updatePersistenceStatus(ctx context.Context, id string, state persistence.TaskState) error {
	parentID := ParentWorkflowID(ctx)
	var parentIDPtr *string
	if parentID != "" {
		parentIDPtr = &parentID
	}

	status := persistence.WorkflowStatus{
		WorkflowID:       id,
		WorkflowName:     o.workflow.Name,
		WorkflowState:    state,
		Timestamp:        time.Now().UTC(),
		NonRestorable:    IsNonRestorable(ctx),
		ParentWorkflowID: parentIDPtr,
	}

	return o.persister.LogWorkflowStatus(ctx, status)
}

var ErrPanic = errors.New("panic in activity")

// executeNode executes a single node and returns its output.
func (o *Orchestrator) executeNode(ctx context.Context, node *Node, input []byte) (output []byte, activityError error) {
	// Defer a function to recover from panics and log the panic details.
	defer func() {
		if r := recover(); r != nil {
			// Log the panic and return a generic error
			Logger(ctx).Error("panic in executeNode", "node", node.ActivityName, "panic", r)
			o.logWorkflowStep(ctx, node, input, nil, fmt.Errorf("panic: %v", r), persistence.StatePanicked)
			activityError = fmt.Errorf("activity %s: panic occurred: %v: %w", node.ActivityName, r, ErrPanic)
			output = nil
		}
	}()

	ctx = node.withActivityStartTime(ctx, time.Now().UTC())

	activity, ok := o.GetActivity(node)
	if !ok {
		return nil, fmt.Errorf("activity %s: %w", node.ActivityName, ErrOrchestratorActivityNotFound)
	}

	ctx1 := o.withNodeContext(ctx, node)
	ctx1 = withActivityToken(ctx1)

	var dynamicRoute *DynamicRoute
	attempt := 1
	maxAttempts := node.RetryPolicyOrDefault(o.defaultRetryPolicy).MaxRetries
	if maxAttempts == 0 {
		maxAttempts = math.MaxInt
	}

	for attempt <= maxAttempts {
		// check if the context has been cancelled
		select {
		case <-ctx1.Done():
			o.logWorkflowStep(ctx1, node, input, output, activityError, persistence.StateTimedOut)
			return nil, ctx1.Err()
		default:
		}

		// sandwitch the activity between logging
		o.logWorkflowStep(ctx1, node, input, output, activityError, persistence.StateOpen)
		output, activityError = activity(ctx1, input)

		state := persistence.StateCompleted
		if activityError != nil && (!errors.As(activityError, &dynamicRoute) || (dynamicRoute.Err != nil)) {
			state = persistence.StateFailed
		}
		o.logWorkflowStep(ctx1, node, input, output, activityError, state)

		// abort if the activity is successful or the error is not retryable
		if state == persistence.StateCompleted || !node.IsRetryableError(activityError) {
			break
		}

		// If retries are exhausted, break
		if attempt == maxAttempts {
			break
		}

		delay := node.RetryPolicyOrDefault(o.defaultRetryPolicy).backoff(attempt)
		Logger(ctx).Error("retrying activity after failurep", "activity", node.ActivityName, "attempt", attempt, "delay", delay, "error", activityError)
		<-time.After(delay)
		attempt++
	}

	if errors.As(activityError, &dynamicRoute) && dynamicRoute.Err != nil {
		return nil, fmt.Errorf("activity %s: %w", node.ActivityName, dynamicRoute.Err)
	} else if !errors.As(activityError, &dynamicRoute) && activityError != nil {
		return nil, fmt.Errorf("activity %s: %w", node.ActivityName, activityError)
	}

	return output, activityError
}

func (o *Orchestrator) logWorkflowStep(ctx context.Context, node *Node, input, output []byte, err error, state persistence.TaskState) {
	if o.persister == nil {
		return
	}

	id := WorkflowID(ctx)
	if id == "" {
		Logger(ctx).Error("persister present but no workflow ID set")
		return
	}

	ts := node.ActivityStartTime(ctx)
	entry := &persistence.WorkflowLogEntry{
		WorkflowID:    id,
		WorkflowName:  o.workflow.Name,
		ActivityToken: ActivityToken(ctx),
		NodeID:        node.ID,
		ActivityName:  node.ActivityName,
		Input:         input,
		Output:        output,
		Timestamp:     ts,
		Duration:      time.Since(ts),
		ActivityState: state,
	}

	if node.Config != nil {
		config, err := json.Marshal(node.Config)
		if err != nil {
			Logger(ctx).Error("failed to marshal config", "error", err)
		}
		configString := string(config)
		entry.Config = &configString
	}

	var dynamicRoute *DynamicRoute
	if err != nil && errors.As(err, &dynamicRoute) {
		errJSON, _ := json.Marshal(err)
		errJSONString := string(errJSON)
		entry.Error = &errJSONString
		if dynamicRoute.Err != nil {
			entry.ActivityState = persistence.StateFailed
		}
	} else if err != nil {
		errString := err.Error()
		entry.Error = &errString
		entry.ActivityState = persistence.StateFailed
	}

	o.tryLogWorkflowStep(ctx, entry)
}

func (o *Orchestrator) tryLogWorkflowStep(ctx context.Context, entry *persistence.WorkflowLogEntry) {
	p := DefaultRetryPolicy()
	p.MaxRetries = math.MaxInt

	// Retry loop for persistence logging errors
	for i := 0; ; i++ {
		if err := o.persister.LogWorkflowStep(ctx, entry); err == nil {
			break
		}

		delay := p.backoff(i)
		Logger(ctx).Error("failed to log workflow step", "attempt", i, "delay", delay)
		<-time.After(delay)
	}
}

type RestorableWorkflowState struct {
	Status     *persistence.WorkflowStatus
	Entrypoint func(ctx context.Context) ([]byte, error)
}

// RestorableWorkflows returns a map of workflowName to a map of workflowID to
// a an array of node functions that allow the client to restore them.
type RestorableWorkflows map[string]map[string]RestorableWorkflowState

func (o *Orchestrator) RestoreWorkflowsAsync(ctx context.Context, withChildWorkflows bool) error {
	restorable, err := o.RestorableWorkflows(ctx)
	if err != nil {
		return err
	}

	for _, workflows := range restorable {
		for id, restorable := range workflows {
			go func(id string, state RestorableWorkflowState) {
				if state.Status.ParentWorkflowID != nil && !withChildWorkflows {
					return
				}
				if _, err := state.Entrypoint(ctx); err != nil {
					Logger(ctx).Error("failed to restore workflow", "workflow", id, "error", err)
				}
			}(id, restorable)
		}
	}

	return nil
}

func (o *Orchestrator) RestorableWorkflows(ctx context.Context) (RestorableWorkflows, error) {
	if o.persister == nil {
		return nil, ErrOrchestratorHasNoPersister
	}

	workflows, err := o.persister.LoadOpenWorkflows(ctx, o.workflow.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to load open workflows: %w", err)
	}

	restorable := make(RestorableWorkflows)
	for _, workflow := range workflows {
		// Restore the workflow with a unique context
		ctx1 := WithWorkflowID(ctx, workflow.WorkflowID)

		// Fail any non-restorable workflows that are still open
		if workflow.NonRestorable {
			ctx1 = WithNonRestorable(ctx1)
			o.logger.Warn("workflow is non-restorable and will be failed", "workflow", workflow.WorkflowID)
			o.tryUpdatePersistenceStatus(ctx1, persistence.StateFailed)
			continue
		}

		// Recover workflow from the last steps
		steps, err := o.persister.LoadWorkflowSteps(ctx, workflow.WorkflowID)
		if err != nil {
			return nil, fmt.Errorf("workflow %s: %w", workflow.WorkflowID, err)
		}

		// Build a map of completed nodes and their outputs
		var initialNodeInput []byte
		completedNodeOutput := make(map[int64][]byte)
		completedNodeErrors := make(map[int64]*DynamicRoute)
		for i, step := range steps {
			// input only matters for the first node, as everything else is path dependent.
			// we only capture it here in case we restore a workflow from the beginning.
			if i == 0 {
				initialNodeInput = steps[0].Input
			}
			if step.ActivityState == persistence.StateCompleted {
				completedNodeOutput[step.NodeID] = step.Output
				var dynamicRoute DynamicRoute
				if step.Error != nil {
					if err := dynamicRoute.UnmarshalJSON([]byte(*step.Error)); err == nil {
						completedNodeErrors[step.NodeID] = &dynamicRoute
					}
				}
			}
		}

		// Determine where to resume execution
		var earliestPendingNode *Node
		// Find predecessor that is completed and not parallel
		for _, node := range o.workflow.Nodes {
			_, parallel := o.parallel[node.ID]
			_, completed := completedNodeOutput[node.ID]
			if !completed || parallel {
				predecessors := o.workflow.directedGraph.To(node.ID)
				var isEarlier bool
				for predecessors.Next() {
					_, parallel := o.parallel[predecessors.Node().ID()]
					_, completed := completedNodeOutput[predecessors.Node().ID()]
					if completed && !parallel && (earliestPendingNode == nil || predecessors.Node().ID() < earliestPendingNode.ID) {
						isEarlier = true
						break
					}
				}
				if isEarlier {
					earliestPendingNode = node
				}
			}
		}

		node := earliestPendingNode
		if node == nil {
			restorable[o.workflow.Name] = make(map[string]RestorableWorkflowState)
			restorable[o.workflow.Name][workflow.WorkflowID] = RestorableWorkflowState{
				Status: workflow,
				Entrypoint: func(ctx context.Context) ([]byte, error) {
					// if the earliest node is nil, we're restoring this workflow from
					// the beginning and have to use the initial node input.
					ctx = WithRestoringMainWorkflow(ctx)
					return o.Start(ctx, initialNodeInput)
				},
			}
			return restorable, nil
		}

		var inputData []byte
		// Collect outputs from predecessor nodes
		predecessors := o.workflow.directedGraph.To(node.ID)
		if predecessors.Len() > 0 {
			var inputs [][]byte
			for predecessors.Next() {
				if output, ok := completedNodeOutput[predecessors.Node().ID()]; ok {
					inputs = append(inputs, output)
				}
			}
			if len(inputs) > 0 {
				// If node has a reducer, apply it to inputs
				if reducer, ok := o.GetReducer(node); ok {
					inputData = reducer(inputs)
				} else {
					// Default behavior, use first input
					inputData = inputs[0]
				}
			}
		}
		if _, ok := restorable[o.workflow.Name]; !ok {
			restorable[o.workflow.Name] = make(map[string]RestorableWorkflowState)
		}
		restorable[o.workflow.Name][workflow.WorkflowID] =
			RestorableWorkflowState{
				Status: workflow,
				Entrypoint: func(ctx context.Context) ([]byte, error) {
					return o.executeNodeChain(ctx, node, inputData)
				},
			}
	}

	return restorable, nil
}
