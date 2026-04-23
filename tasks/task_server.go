package tasks

import (
	"log/slog"
	"time"

	"echolist-backend/common"
	pb "echolist-backend/proto/gen/tasks/v1"
	tasksv1connect "echolist-backend/proto/gen/tasks/v1/tasksv1connect"
)

// TaskServer implements the TaskListService RPC handler.
type TaskServer struct {
	tasksv1connect.UnimplementedTaskListServiceHandler
	dataDir string
	locks   common.Locker
	logger  *slog.Logger
}

// NewTaskServer creates a new TaskServer rooted at dataDir.
func NewTaskServer(dataDir string, logger *slog.Logger) *TaskServer {
	return &TaskServer{dataDir: dataDir, logger: logger.With("service", "tasks")}
}

// protoToMainTasks converts proto MainTask messages to domain types.
func protoToMainTasks(pbTasks []*pb.MainTask) []MainTask {
	tasks := make([]MainTask, len(pbTasks))
	for i, pt := range pbTasks {
		tasks[i] = MainTask{
			Id:          pt.Id,
			Description: pt.Description,
			IsDone:      pt.IsDone,
			DueDate:     pt.DueDate,
			Recurrence:  pt.Recurrence,
			SubTasks:    protoToSubtasks(pt.SubTasks),
		}
	}
	return tasks
}

func protoToSubtasks(pbSubs []*pb.SubTask) []SubTask {
	if len(pbSubs) == 0 {
		return nil
	}
	subs := make([]SubTask, len(pbSubs))
	for i, ps := range pbSubs {
		subs[i] = SubTask{Id: ps.Id, Description: ps.Description, IsDone: ps.IsDone}
	}
	return subs
}

// mainTasksToProto converts domain types to proto MainTask messages.
func mainTasksToProto(tasks []MainTask) []*pb.MainTask {
	pbTasks := make([]*pb.MainTask, len(tasks))
	for i, t := range tasks {
		pbTasks[i] = &pb.MainTask{
			Id:          t.Id,
			Description: t.Description,
			IsDone:      t.IsDone,
			DueDate:     t.DueDate,
			Recurrence:  t.Recurrence,
			SubTasks:    subtasksToProto(t.SubTasks),
		}
	}
	return pbTasks
}

// buildTaskList constructs a pb.TaskList from the given parameters.
func buildTaskList(id, parentDir, title string, tasks []MainTask, updatedAt int64, isAutoDelete bool) *pb.TaskList {
	return &pb.TaskList{
		Id:           id,
		ParentDir:    parentDir,
		Title:        title,
		Tasks:        mainTasksToProto(tasks),
		UpdatedAt:    updatedAt,
		IsAutoDelete: isAutoDelete,
	}
}


func subtasksToProto(subs []SubTask) []*pb.SubTask {
	if len(subs) == 0 {
		return nil
	}
	pbSubs := make([]*pb.SubTask, len(subs))
	for i, s := range subs {
		pbSubs[i] = &pb.SubTask{Id: s.Id, Description: s.Description, IsDone: s.IsDone}
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
	return common.ExtractTitle(filename, common.TaskListFileType.Prefix, common.TaskListFileType.Suffix, common.TaskListFileType.Label)
}
