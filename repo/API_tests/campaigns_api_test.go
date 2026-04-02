package API_tests

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"parkops/internal/campaigns"
)

func TestCampaignTaskReminderStopsAfterComplete(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	topics := apiRequest(t, env.r, http.MethodGet, "/api/notification-topics", nil, admin)
	logStep(t, "GET", "/api/notification-topics", topics.Code, topics.Body.String())
	if topics.Code != http.StatusOK {
		t.Fatalf("topics failed: %d %s", topics.Code, topics.Body.String())
	}
	topicID := topicIDByName(t, topics.Body.String(), "task_reminder")

	sub := apiRequest(t, env.r, http.MethodPost, "/api/notification-topics/"+topicID+"/subscribe", nil, admin)
	logStep(t, "POST", "/api/notification-topics/:id/subscribe", sub.Code, sub.Body.String())
	if sub.Code != http.StatusOK {
		t.Fatalf("subscribe failed: %d %s", sub.Code, sub.Body.String())
	}

	createCampaign := apiRequest(t, env.r, http.MethodPost, "/api/campaigns", map[string]any{
		"title":       "Ops Checks",
		"description": "nightly checks",
	}, admin)
	logStep(t, "POST", "/api/campaigns", createCampaign.Code, createCampaign.Body.String())
	if createCampaign.Code != http.StatusCreated {
		t.Fatalf("create campaign failed: %d %s", createCampaign.Code, createCampaign.Body.String())
	}
	campaignID := extractID(t, createCampaign.Body.String())

	createTask := apiRequest(t, env.r, http.MethodPost, "/api/campaigns/"+campaignID+"/tasks", map[string]any{
		"description":               "Check gate A",
		"deadline":                  time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339),
		"reminder_interval_minutes": 1,
	}, admin)
	logStep(t, "POST", "/api/campaigns/:id/tasks", createTask.Code, createTask.Body.String())
	if createTask.Code != http.StatusCreated {
		t.Fatalf("create task failed: %d %s", createTask.Code, createTask.Body.String())
	}
	taskID := extractID(t, createTask.Body.String())

	processor := campaigns.NewService(env.pool)
	if err := processor.ProcessDueTaskReminders(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("process reminders failed: %v", err)
	}

	listNotifications := apiRequest(t, env.r, http.MethodGet, "/api/notifications", nil, admin)
	logStep(t, "GET", "/api/notifications", listNotifications.Code, listNotifications.Body.String())
	if listNotifications.Code != http.StatusOK || !strings.Contains(listNotifications.Body.String(), "Task reminder") {
		t.Fatalf("expected task reminder notification, got %d %s", listNotifications.Code, listNotifications.Body.String())
	}

	var beforeCount int
	err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM notification_jobs WHERE payload->>'task_id'=$1
	`, taskID).Scan(&beforeCount)
	if err != nil {
		t.Fatalf("count reminder jobs before complete: %v", err)
	}
	if beforeCount == 0 {
		t.Fatal("expected at least one reminder job before task completion")
	}

	complete := apiRequest(t, env.r, http.MethodPost, "/api/tasks/"+taskID+"/complete", nil, admin)
	logStep(t, "POST", "/api/tasks/:id/complete", complete.Code, complete.Body.String())
	if complete.Code != http.StatusOK {
		t.Fatalf("complete task failed: %d %s", complete.Code, complete.Body.String())
	}

	if err := processor.ProcessDueTaskReminders(context.Background(), time.Now().UTC().Add(3*time.Minute)); err != nil {
		t.Fatalf("process reminders after complete failed: %v", err)
	}

	var afterCount int
	err = env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM notification_jobs WHERE payload->>'task_id'=$1
	`, taskID).Scan(&afterCount)
	if err != nil {
		t.Fatalf("count reminder jobs after complete: %v", err)
	}
	if afterCount != beforeCount {
		t.Fatalf("expected reminder job count unchanged after completion, before=%d after=%d", beforeCount, afterCount)
	}
}

func TestCampaignAndTaskCRUDEndpoints(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	createCampaign := apiRequest(t, env.r, http.MethodPost, "/api/campaigns", map[string]any{
		"title":       "Morning Ops",
		"description": "open checklist",
	}, admin)
	logStep(t, "POST", "/api/campaigns", createCampaign.Code, createCampaign.Body.String())
	if createCampaign.Code != http.StatusCreated {
		t.Fatalf("create campaign failed: %d %s", createCampaign.Code, createCampaign.Body.String())
	}
	campaignID := extractID(t, createCampaign.Body.String())

	getCampaign := apiRequest(t, env.r, http.MethodGet, "/api/campaigns/"+campaignID, nil, admin)
	logStep(t, "GET", "/api/campaigns/:id", getCampaign.Code, getCampaign.Body.String())
	if getCampaign.Code != http.StatusOK {
		t.Fatalf("get campaign failed: %d %s", getCampaign.Code, getCampaign.Body.String())
	}

	patchCampaign := apiRequest(t, env.r, http.MethodPatch, "/api/campaigns/"+campaignID, map[string]any{
		"description": "open checklist updated",
	}, admin)
	logStep(t, "PATCH", "/api/campaigns/:id", patchCampaign.Code, patchCampaign.Body.String())
	if patchCampaign.Code != http.StatusOK {
		t.Fatalf("patch campaign failed: %d %s", patchCampaign.Code, patchCampaign.Body.String())
	}

	createTask := apiRequest(t, env.r, http.MethodPost, "/api/campaigns/"+campaignID+"/tasks", map[string]any{
		"description":               "Check zone lights",
		"reminder_interval_minutes": 10,
	}, admin)
	logStep(t, "POST", "/api/campaigns/:id/tasks", createTask.Code, createTask.Body.String())
	if createTask.Code != http.StatusCreated {
		t.Fatalf("create task failed: %d %s", createTask.Code, createTask.Body.String())
	}
	taskID := extractID(t, createTask.Body.String())

	listTasks := apiRequest(t, env.r, http.MethodGet, "/api/campaigns/"+campaignID+"/tasks", nil, admin)
	logStep(t, "GET", "/api/campaigns/:id/tasks", listTasks.Code, listTasks.Body.String())
	if listTasks.Code != http.StatusOK || !strings.Contains(listTasks.Body.String(), taskID) {
		t.Fatalf("list tasks failed: %d %s", listTasks.Code, listTasks.Body.String())
	}

	patchTask := apiRequest(t, env.r, http.MethodPatch, "/api/tasks/"+taskID, map[string]any{
		"description": "Check zone lights and signage",
	}, admin)
	logStep(t, "PATCH", "/api/tasks/:id", patchTask.Code, patchTask.Body.String())
	if patchTask.Code != http.StatusOK {
		t.Fatalf("patch task failed: %d %s", patchTask.Code, patchTask.Body.String())
	}

	getTask := apiRequest(t, env.r, http.MethodGet, "/api/tasks/"+taskID, nil, admin)
	logStep(t, "GET", "/api/tasks/:id", getTask.Code, getTask.Body.String())
	if getTask.Code != http.StatusOK || !strings.Contains(getTask.Body.String(), "signage") {
		t.Fatalf("get task failed: %d %s", getTask.Code, getTask.Body.String())
	}

	deleteTask := apiRequest(t, env.r, http.MethodDelete, "/api/tasks/"+taskID, nil, admin)
	logStep(t, "DELETE", "/api/tasks/:id", deleteTask.Code, deleteTask.Body.String())
	if deleteTask.Code != http.StatusNoContent {
		t.Fatalf("delete task failed: %d %s", deleteTask.Code, deleteTask.Body.String())
	}

	deleteCampaign := apiRequest(t, env.r, http.MethodDelete, "/api/campaigns/"+campaignID, nil, admin)
	logStep(t, "DELETE", "/api/campaigns/:id", deleteCampaign.Code, deleteCampaign.Body.String())
	if deleteCampaign.Code != http.StatusNoContent {
		t.Fatalf("delete campaign failed: %d %s", deleteCampaign.Code, deleteCampaign.Body.String())
	}
}

func TestCampaignTaskReminderHonorsDND(t *testing.T) {
	env := setupAuthAPIEnv(t)
	admin := loginAs(t, env, "admin", "AdminPass1234")

	topics := apiRequest(t, env.r, http.MethodGet, "/api/notification-topics", nil, admin)
	logStep(t, "GET", "/api/notification-topics", topics.Code, topics.Body.String())
	if topics.Code != http.StatusOK {
		t.Fatalf("topics failed: %d %s", topics.Code, topics.Body.String())
	}
	topicID := topicIDByName(t, topics.Body.String(), "task_reminder")

	sub := apiRequest(t, env.r, http.MethodPost, "/api/notification-topics/"+topicID+"/subscribe", nil, admin)
	logStep(t, "POST", "/api/notification-topics/:id/subscribe", sub.Code, sub.Body.String())
	if sub.Code != http.StatusOK {
		t.Fatalf("subscribe failed: %d %s", sub.Code, sub.Body.String())
	}

	setDND := apiRequest(t, env.r, http.MethodPatch, "/api/notification-settings/dnd", map[string]any{
		"start_time": "00:00",
		"end_time":   "23:59",
		"enabled":    true,
	}, admin)
	logStep(t, "PATCH", "/api/notification-settings/dnd", setDND.Code, setDND.Body.String())
	if setDND.Code != http.StatusOK {
		t.Fatalf("set dnd failed: %d %s", setDND.Code, setDND.Body.String())
	}

	createCampaign := apiRequest(t, env.r, http.MethodPost, "/api/campaigns", map[string]any{
		"title":       "DND Ops Checks",
		"description": "dnd task reminders",
	}, admin)
	logStep(t, "POST", "/api/campaigns", createCampaign.Code, createCampaign.Body.String())
	if createCampaign.Code != http.StatusCreated {
		t.Fatalf("create campaign failed: %d %s", createCampaign.Code, createCampaign.Body.String())
	}
	campaignID := extractID(t, createCampaign.Body.String())

	createTask := apiRequest(t, env.r, http.MethodPost, "/api/campaigns/"+campaignID+"/tasks", map[string]any{
		"description":               "Check gate B",
		"deadline":                  time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339),
		"reminder_interval_minutes": 1,
	}, admin)
	logStep(t, "POST", "/api/campaigns/:id/tasks", createTask.Code, createTask.Body.String())
	if createTask.Code != http.StatusCreated {
		t.Fatalf("create task failed: %d %s", createTask.Code, createTask.Body.String())
	}
	taskID := extractID(t, createTask.Body.String())

	processor := campaigns.NewService(env.pool)
	if err := processor.ProcessDueTaskReminders(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("process reminders failed: %v", err)
	}

	var deferred int
	err := env.pool.QueryRow(context.Background(), `
		SELECT COUNT(*)
		FROM notification_jobs
		WHERE payload->>'task_id'=$1
		  AND status='deferred'
		  AND next_attempt_at IS NOT NULL
	`, taskID).Scan(&deferred)
	if err != nil {
		t.Fatalf("query deferred task reminders: %v", err)
	}
	if deferred == 0 {
		t.Fatal("expected deferred task reminder job while DND is active")
	}
}
