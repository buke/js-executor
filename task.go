// Copyright 2025 Brian Wang <wangbuke@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package jsexecutor

// taskStatus represents the current status of a task.
type taskStatus int

const (
	taskStatusPending   taskStatus = iota // Task is waiting to be executed
	taskStatusRunning                     // Task is currently being executed
	taskStatusCompleted                   // Task execution has completed
)

// taskResult represents the result of task execution.
type taskResult struct {
	response *JsResponse // JavaScript execution response (nil if error occurred)
	err      error       // Error that occurred during execution (nil if successful)
}

// task represents a unit of work to be executed by a thread.
type task struct {
	request    *JsRequest       // JavaScript request to execute
	resultChan chan *taskResult // Channel to receive the execution result
	status     taskStatus       // Current status of the task
}

// newTask creates a new task instance for the given request.
func newTask(request *JsRequest) *task {
	return &task{
		request:    request,
		resultChan: make(chan *taskResult, 1), // Buffered channel to prevent blocking
		status:     taskStatusPending,
	}
}
