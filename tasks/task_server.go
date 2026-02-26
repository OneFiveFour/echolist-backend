package tasks

import (
	"os"
	"path/filepath"
	"time"

	pb "echolist-backend/proto/gen/tasks/v1"
	tasksv1connect "echolist-backend/proto/gen/tasks/v1/tasksv1connect"
)

// TaskServer implements the TasksService RPC handler.
type TaskServer struct {
	tasksv1connect.UnimplementedTasksServiceHandler
	dataDir string
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

// atomicWriteFile writes data to path atomically via temp file + rename.
func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// nowMillis returns the current time in Unix milliseconds.
func nowMillis() int64 {
	return time.Now().UnixMilli()
}
