package tasks

import (
	"time"

	"echolist-backend/pathlock"
	"echolist-backend/pathutil"
	pb "echolist-backend/proto/gen/tasks/v1"
	tasksv1connect "echolist-backend/proto/gen/tasks/v1/tasksv1connect"
)

// TaskServer implements the TaskListService RPC handler.
type TaskServer struct {
	tasksv1connect.UnimplementedTaskListServiceHandler
	dataDir string
	locks   pathlock.Locker
}

// NewTaskServer creates a new TaskServer rooted at dataDir.
func NewTaskServer(dataDir string) *TaskServer {
	return &TaskServer{dataDir: dataDir}
}

// protoToMainTasks converts proto MainTask messages to domain types.
func protoToMainTasks(pbTasks []*pb.MainTask) []MainTask {
	tasks := make([]MainTask, len(pbTasks))
	for i, pt := range pbTasks {
		tasks[i] = MainTask{
			Description: pt.Description,
			Done:        pt.Done,
			DueDate:     pt.DueDate,
			Recurrence:  pt.Recurrence,
			Subtasks:    protoToSubtasks(pt.Subtasks),
		}
	}
	return tasks
}

func protoToSubtasks(pbSubs []*pb.Subtask) []Subtask {
	if len(pbSubs) == 0 {
		return nil
	}
	subs := make([]Subtask, len(pbSubs))
	for i, ps := range pbSubs {
		subs[i] = Subtask{Description: ps.Description, Done: ps.Done}
	}
	return subs
}

// mainTasksToProto converts domain types to proto MainTask messages.
func mainTasksToProto(tasks []MainTask) []*pb.MainTask {
	pbTasks := make([]*pb.MainTask, len(tasks))
	for i, t := range tasks {
		pbTasks[i] = &pb.MainTask{
			Description: t.Description,
			Done:        t.Done,
			DueDate:     t.DueDate,
			Recurrence:  t.Recurrence,
			Subtasks:    subtasksToProto(t.Subtasks),
		}
	}
	return pbTasks
}

// buildTaskList constructs a pb.TaskList from the given parameters.
func buildTaskList(filePath, title string, tasks []MainTask, updatedAt int64) *pb.TaskList {
	return &pb.TaskList{
		FilePath:  filePath,
		Title:     title,
		Tasks:     mainTasksToProto(tasks),
		UpdatedAt: updatedAt,
	}
}


func subtasksToProto(subs []Subtask) []*pb.Subtask {
	if len(subs) == 0 {
		return nil
	}
	pbSubs := make([]*pb.Subtask, len(subs))
	for i, s := range subs {
		pbSubs[i] = &pb.Subtask{Description: s.Description, Done: s.Done}
	}
	return pbSubs
}

// nowMillis returns the current time in Unix milliseconds.
func nowMillis() int64 {
	return time.Now().UnixMilli()
}
// ExtractTaskListTitle extracts the human-readable title from a task-list
// filename (e.g. "tasks_Shopping.md" → "Shopping").
// Returns an error if the filename is too short or doesn't match the expected pattern.
func ExtractTaskListTitle(filename string) (string, error) {
	return pathutil.ExtractTitle(filename, pathutil.TaskListFileType.Prefix, pathutil.TaskListFileType.Suffix, pathutil.TaskListFileType.Label)
}
