package notify

import "testing"

func TestBuildBrokerControlCardShowsConnectWhenDisconnected(t *testing.T) {
	card := BuildBrokerControlCard(BrokerControlStatus{
		Profile:        "claude-main",
		Agent:          "claude",
		PermissionMode: "workspace-write",
	})

	header := card["header"].(map[string]any)
	if got := header["template"]; got != "grey" {
		t.Fatalf("header template = %v, want grey", got)
	}
	if !cardHasButtonAction(card, "broker_connect") {
		t.Fatal("control card should expose broker_connect action")
	}
}

func TestBuildBrokerControlCardShowsDisconnectWhenConnected(t *testing.T) {
	card := BuildBrokerControlCard(BrokerControlStatus{
		Profile:        "claude-main",
		Agent:          "claude",
		PermissionMode: "workspace-write",
		BrokerEnabled:  true,
		ProfileEnabled: true,
	})

	header := card["header"].(map[string]any)
	if got := header["template"]; got != "green" {
		t.Fatalf("header template = %v, want green", got)
	}
	if !cardHasButtonAction(card, "broker_disconnect") {
		t.Fatal("control card should expose broker_disconnect action")
	}
	if !cardHasButtonAction(card, "broker_pause") {
		t.Fatal("control card should expose broker_pause action")
	}
	if !cardHasButtonAction(card, "broker_stop") {
		t.Fatal("control card should expose broker_stop action")
	}
}

func TestBuildThreadListCardHasNavigationActions(t *testing.T) {
	card := BuildThreadListCard(ThreadListStatus{
		Profile:   "claude-main",
		Workspace: "/repo",
		Page:      2,
		HasPrev:   true,
		HasNext:   true,
		Threads: []ThreadSummary{{
			ID:        "th_1",
			Number:    1,
			Title:     "测试",
			Status:    "complete",
			UpdatedAt: "07-05 12:00",
		}},
	})
	for _, action := range []string{"thread_open", "thread_result", "thread_new", "home", "threads_list"} {
		if !cardHasButtonAction(card, action) {
			t.Fatalf("thread list card should expose %s action", action)
		}
	}
}

func TestBuildTaskStatusCardHasReturnAndOutputActions(t *testing.T) {
	card := BuildTaskStatusCard(TaskStatus{
		Profile:   "claude-main",
		Workspace: "/repo",
		ThreadID:  "th_1",
		TaskID:    "task_1",
		Number:    1,
		Status:    "running",
		Progress:  "working",
	})
	if !cardTextContains(card, "模型输出中") {
		t.Fatal("running task card should label progress as model streaming output")
	}
	for _, action := range []string{"task_tail", "task_log", "thread_open", "home", "task_stop"} {
		if !cardHasButtonAction(card, action) {
			t.Fatalf("task card should expose %s action", action)
		}
	}
}

func TestBuildDoneTaskStatusCardUsesModelOutputAction(t *testing.T) {
	card := BuildTaskStatusCard(TaskStatus{
		Profile:   "claude-main",
		Workspace: "/repo",
		ThreadID:  "th_1",
		TaskID:    "task_1",
		Number:    1,
		Status:    "done",
		Progress:  "不应重复展示的预览",
		Final:     "模型最终输出",
	})
	if !cardHasButtonAction(card, "task_result") {
		t.Fatal("done task card should expose task_result action")
	}
	for _, action := range []string{"task_tail", "task_log"} {
		if cardHasButtonAction(card, action) {
			t.Fatalf("done task card should not expose %s action", action)
		}
	}
	if !cardTextContains(card, "模型最终输出") {
		t.Fatal("done task card should include final model output")
	}
	if cardTextContains(card, "不应重复展示的预览") {
		t.Fatal("done task card should not duplicate progress when final output exists")
	}
}

func cardHasButtonAction(card map[string]any, action string) bool {
	elements, _ := card["elements"].([]any)
	for _, item := range elements {
		element, _ := item.(map[string]any)
		actions, _ := element["actions"].([]any)
		for _, rawAction := range actions {
			button, _ := rawAction.(map[string]any)
			value, _ := button["value"].(map[string]any)
			if value["action"] == action {
				return true
			}
		}
	}
	return false
}

func cardTextContains(card map[string]any, want string) bool {
	elements, _ := card["elements"].([]any)
	for _, item := range elements {
		element, _ := item.(map[string]any)
		text, _ := element["text"].(map[string]any)
		content, _ := text["content"].(string)
		if contains(content, want) {
			return true
		}
	}
	return false
}
