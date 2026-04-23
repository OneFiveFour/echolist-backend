package tasks

import (
	"log/slog"
	"time"

	"echolist-backend/database"
	pb "echolist-backend/proto/gen/tasks/v1"
	tasksv1connect "echolist-backend/proto/gen/tasks/v1/tasksv1connect"
)

// TaskServer implements the TaskListService RPC handler.
type TaskServer struct {
	tasksv1connect.UnimplementedTaskListServiceHandler
	dataDir string
	db      *database.Database
	logger  *slog.Logger
}

// NewTaskServer creates a new TaskServer rooted at dataDir.
func NewTaskServer(dataDir string, db *database.Database, logger *slog.Logger) *TaskServer {
	return &TaskServer{dataDir: dataDir, db: db, logger: logger.With("service", "tasks")}
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

// taskRowsToMainTasks converts database TaskRows into domain MainTask slices.
// Main tasks have TaskListId set; subtasks have ParentTaskId set.
// Subtasks are grouped under their parent main task by ParentTaskId.
func taskRowsToMainTasks(rows []database.TaskRow) []MainTask {
	// First pass: build main tasks in order, index by ID.
	mainTaskMap := make(map[string]int) // mainTask ID → index in result
	var result []MainTask

	for _, r := range rows {
		if r.TaskListId != nil {
			// This is a main task.
			mt := MainTask{
				Id:          r.Id,
				Description: r.Description,
				IsDone:      r.IsDone,
			}
			if r.DueDate != nil {
				mt.DueDate = *r.DueDate
			}
			if r.Recurrence != nil {
				mt.Recurrence = *r.Recurrence
			}
			mainTaskMap[r.Id] = len(result)
			result = append(result, mt)
		}
	}

	// Second pass: attach subtasks to their parent main tasks.
	for _, r := range rows {
		if r.ParentTaskId != nil {
			st := SubTask{
				Id:          r.Id,
				Description: r.Description,
				IsDone:      r.IsDone,
			}
			if idx, ok := mainTaskMap[*r.ParentTaskId]; ok {
				result[idx].SubTasks = append(result[idx].SubTasks, st)
			}
		}
	}

	return result
}
