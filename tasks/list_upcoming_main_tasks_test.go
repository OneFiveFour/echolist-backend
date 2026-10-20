package tasks_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"echolist-backend/auth"
	"echolist-backend/database"
	pb "echolist-backend/proto/gen/tasks/v1"
	tasksv1connect "echolist-backend/proto/gen/tasks/v1/tasksv1connect"
	"echolist-backend/tasks"
)

func TestListUpcomingMainTasks_DateValidation(t *testing.T) {
	t.Parallel()
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	for _, value := range []string{
		"", "2026-2-03", "2026-02-3", "2026-02-29", "2026-04-31",
		"2026-13-01", "2026-00-01", "2026-10-00", " 2026-10-04",
		"2026-10-04 ", "2026-10-04T00:00:00Z", "2026-10-04+02:00", "invalid",
	} {
		t.Run(value, func(t *testing.T) {
			_, err := srv.ListUpcomingMainTasks(context.Background(), &pb.ListUpcomingMainTasksRequest{FromDate: value})
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("expected invalid_argument for %q, got %v", value, err)
			}
		})
	}
	for _, value := range []string{"2024-02-29", "2026-01-01", "2026-12-31"} {
		resp, err := srv.ListUpcomingMainTasks(context.Background(), &pb.ListUpcomingMainTasksRequest{FromDate: value})
		if err != nil || len(resp.GetTasks()) != 0 {
			t.Fatalf("valid date %q: response %v, err %v", value, resp, err)
		}
	}
}

func TestListUpcomingMainTasks_Errors(t *testing.T) {
	t.Parallel()
	db := tasks.NewTestDB(t)
	srv := tasks.NewTaskServer(t.TempDir(), db, tasks.NopLogger())
	req := &pb.ListUpcomingMainTasksRequest{FromDate: "2026-10-04"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := srv.ListUpcomingMainTasks(ctx, req); connect.CodeOf(err) != connect.CodeCanceled {
		t.Fatalf("expected canceled, got %v", err)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	if _, err := srv.ListUpcomingMainTasks(ctx, req); connect.CodeOf(err) != connect.CodeDeadlineExceeded {
		t.Fatalf("expected deadline_exceeded, got %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	resp, err := srv.ListUpcomingMainTasks(context.Background(), req)
	if resp != nil || connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("expected internal error with no response, got %v, %v", resp, err)
	}
	if err.Error() != "internal: failed to list upcoming main tasks" {
		t.Fatalf("internal details exposed: %v", err)
	}
}

func TestListUpcomingMainTasks_HttpContract(t *testing.T) {
	t.Parallel()
	db := tasks.NewTestDB(t)
	dueDate := "2026-10-04"
	const taskId = "00000000-0000-4000-a000-000000000001"
	if _, _, err := db.CreateTaskList(database.CreateTaskListParams{
		Id: "list", Title: "List", ParentDir: "Work/Projects",
		Tasks: []database.CreateTaskParams{
			{Id: taskId, Description: "Call dentist", DueDate: &dueDate,
				SubTasks: []database.CreateTaskParams{{Id: "sub", Description: "Find phone number"}}},
			{Id: "done", Description: "Already completed", IsDone: true, DueDate: &dueDate},
		},
	}); err != nil {
		t.Fatal(err)
	}
	logger := tasks.NopLogger()
	tokens := auth.NewTokenService("test-secret", time.Hour, time.Hour)
	accessToken, err := tokens.GenerateAccessToken("testuser")
	if err != nil {
		t.Fatal(err)
	}
	refreshToken, err := tokens.GenerateRefreshToken("testuser")
	if err != nil {
		t.Fatal(err)
	}
	path, handler := tasksv1connect.NewTaskListServiceHandler(
		tasks.NewTaskServer(t.TempDir(), db, logger),
		connect.WithInterceptors(auth.NewAuthInterceptor(tokens, logger)),
	)
	mux := http.NewServeMux()
	mux.Handle(path, handler)

	for _, tc := range []struct {
		name   string
		token  string
		body   string
		status int
	}{
		{"missing token", "", `{"fromDate":"2026-10-04"}`, http.StatusUnauthorized},
		{"invalid token", "invalid", `{"fromDate":"2026-10-04"}`, http.StatusUnauthorized},
		{"refresh token", refreshToken, `{"fromDate":"2026-10-04"}`, http.StatusUnauthorized},
		{"missing cutoff", accessToken, `{}`, http.StatusBadRequest},
		{"UTC timestamp instead of date", accessToken, `{"fromDate":"2026-10-04T00:00:00Z"}`, http.StatusBadRequest},
		{"upcoming", accessToken, `{"fromDate":"2026-10-04"}`, http.StatusOK},
		{"empty", accessToken, `{"fromDate":"2026-10-05"}`, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/tasks.v1.TaskListService/ListUpcomingMainTasks", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Connect-Protocol-Version", "1")
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if tc.status != http.StatusOK {
				return
			}
			var got struct {
				Tasks []map[string]any `json:"tasks"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if tc.name == "empty" {
				if len(got.Tasks) != 0 {
					t.Fatalf("expected empty results, got %v", got.Tasks)
				}
				return
			}
			want := []map[string]any{{"taskId": taskId, "tasklistId": "list", "title": "Call dentist", "dueDate": dueDate}}
			if !reflect.DeepEqual(got.Tasks, want) {
				t.Fatalf("expected only notification fields %v, got %v", want, got.Tasks)
			}
		})
	}

	// Exercise the generated client using binary Protobuf, as native clients do.
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()
	client := tasksv1connect.NewTaskListServiceClient(httpServer.Client(), httpServer.URL,
		connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				req.Header().Set("Authorization", "Bearer "+accessToken)
				return next(ctx, req)
			}
		})),
	)
	request := &pb.ListUpcomingMainTasksRequest{FromDate: dueDate}
	response, err := client.ListUpcomingMainTasks(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.GetTasks()) != 1 {
		t.Fatalf("expected one upcoming task, got %v", response.GetTasks())
	}
	if got := response.GetTasks()[0]; got.GetTaskId() != taskId || got.GetTasklistId() != "list" {
		t.Fatalf("unexpected task/list association: %v", got)
	}
}
