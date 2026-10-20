package database_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"echolist-backend/database"
)

func TestListUpcomingMainTasks_FilterAndOrder(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	date := func(value string) *string { return &value }
	for _, list := range []database.CreateTaskListParams{
		{
			Id: "root", Title: "Root", Tasks: []database.CreateTaskParams{
				{Id: "no-date", Description: "No due date"},
				{Id: "empty-date", Description: "Empty due date", DueDate: date("")},
				{Id: "overdue", DueDate: date("2026-10-03"), SubTasks: []database.CreateTaskParams{
					{Id: "future-subtask", DueDate: date("2026-10-05")},
				}},
				{Id: "done-today", IsDone: true, DueDate: date("2026-10-04")},
				{Id: "done-future", IsDone: true, DueDate: date("2026-10-05")},
				{Id: "z-today", Description: "Today Z", DueDate: date("2026-10-04"), SubTasks: []database.CreateTaskParams{
					{Id: "subtask", Description: "Child"},
				}},
				{Id: "future", Description: "Next year", DueDate: date("2027-01-01")},
			},
		},
		{
			Id: "nested", Title: "Nested", ParentDir: "Work/Projects", Tasks: []database.CreateTaskParams{
				{Id: "recurring", Description: "Recurring", DueDate: date("2026-10-05"), Recurrence: date("FREQ=DAILY")},
				{Id: "a-today", Description: "Today A", DueDate: date("2026-10-04")},
			},
		},
	} {
		if _, _, err := db.CreateTaskList(list); err != nil {
			t.Fatal(err)
		}
	}

	got, err := db.ListUpcomingMainTasks(context.Background(), "2026-10-04")
	if err != nil {
		t.Fatal(err)
	}
	want := []database.UpcomingMainTaskRow{
		{TaskId: "a-today", TaskListId: "nested", Title: "Today A", DueDate: "2026-10-04"},
		{TaskId: "z-today", TaskListId: "root", Title: "Today Z", DueDate: "2026-10-04"},
		{TaskId: "recurring", TaskListId: "nested", Title: "Recurring", DueDate: "2026-10-05"},
		{TaskId: "future", TaskListId: "root", Title: "Next year", DueDate: "2027-01-01"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	// Results must reflect task completion and deletion on subsequent requests.
	if _, _, err := db.UpdateTaskList(database.UpdateTaskListParams{
		Id: "root", Title: "Root", Tasks: []database.CreateTaskParams{
			{Id: "z-today", IsDone: true, DueDate: date("2026-10-04")},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DeleteTaskList("nested"); err != nil {
		t.Fatal(err)
	}
	got, err = db.ListUpcomingMainTasks(context.Background(), "2026-10-04")
	if err != nil || len(got) != 0 {
		t.Fatalf("after completion/deletion: got %+v, err %v", got, err)
	}
}

func TestListUpcomingMainTasks_Empty(t *testing.T) {
	t.Parallel()
	got, err := newTestDB(t).ListUpcomingMainTasks(context.Background(), "2026-10-04")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestListUpcomingMainTasks_Canceled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := newTestDB(t).ListUpcomingMainTasks(ctx, "2026-10-04")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
