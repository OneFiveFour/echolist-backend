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
	for i, protoTask := range pbTasks {
		tasks[i] = MainTask{
			Id:          protoTask.Id,
			Description: protoTask.Description,
			IsDone:      protoTask.IsDone,
			DueDate:     protoTask.DueDate,
			Recurrence:  protoTask.Recurrence,
			SubTasks:    protoToSubtasks(protoTask.SubTasks),
		}
	}
	return tasks
}

func protoToSubtasks(pbSubs []*pb.SubTask) []SubTask {
	if len(pbSubs) == 0 {
		return nil
	}
	subs := make([]SubTask, len(pbSubs))
	for i, protoSub := range pbSubs {
		subs[i] = SubTask{Id: protoSub.Id, Description: protoSub.Description, IsDone: protoSub.IsDone}
	}
	return subs
}

// mainTasksToProto converts domain types to proto MainTask messages.
func mainTasksToProto(tasks []MainTask) []*pb.MainTask {
	pbTasks := make([]*pb.MainTask, len(tasks))
	for i, task := range tasks {
		pbTasks[i] = &pb.MainTask{
			Id:          task.Id,
			Description: task.Description,
			IsDone:      task.IsDone,
			DueDate:     task.DueDate,
			Recurrence:  task.Recurrence,
			SubTasks:    subtasksToProto(task.SubTasks),
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
	for i, sub := range subs {
		pbSubs[i] = &pb.SubTask{Id: sub.Id, Description: sub.Description, IsDone: sub.IsDone}
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

	for _, row := range rows {
		if row.TaskListId != nil {
			// This is a main task.
			mainTask := MainTask{
				Id:          row.Id,
				Description: row.Description,
				IsDone:      row.IsDone,
			}
			if row.DueDate != nil {
				mainTask.DueDate = *row.DueDate
			}
			if row.Recurrence != nil {
				mainTask.Recurrence = *row.Recurrence
			}
			mainTaskMap[row.Id] = len(result)
			result = append(result, mainTask)
		}
	}

	// Second pass: attach subtasks to their parent main tasks.
	for _, row := range rows {
		if row.ParentTaskId != nil {
			subTask := SubTask{
				Id:          row.Id,
				Description: row.Description,
				IsDone:      row.IsDone,
			}
			if idx, ok := mainTaskMap[*row.ParentTaskId]; ok {
				result[idx].SubTasks = append(result[idx].SubTasks, subTask)
			}
		}
	}

	return result
}
