package tasks

import (
	"testing"

	"echolist-backend/proto/gen/tasks/v1/tasksv1connect"
)

// TestTaskServerImplementsInterface verifies that TaskServer implements
// the TaskListServiceHandler interface at compile time.
func TestTaskServerImplementsInterface(t *testing.T) {
	// This is a compile-time check. If TaskServer doesn't implement
	// TaskListServiceHandler, this code won't compile.
	var _ tasksv1connect.TaskListServiceHandler = (*TaskServer)(nil)
	
	// If we get here, the interface is properly implemented
	t.Log("TaskServer successfully implements TaskListServiceHandler interface")
}
