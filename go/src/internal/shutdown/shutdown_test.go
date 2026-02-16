package shutdown

import (
	"context"
	"testing"
	"time"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/notifier"
	"github.com/circle-oo/flux/internal/testutil"
	"github.com/google/uuid"
)

// mockPod implements the Pod interface for testing.
type mockPod struct {
	running   bool
	stopChan  chan struct{}
	taskID    string
	stopDelay time.Duration // simulates time to finish work
}

func newMockPod(taskID string, stopDelay time.Duration) *mockPod {
	return &mockPod{
		running:   true,
		stopChan:  make(chan struct{}),
		taskID:    taskID,
		stopDelay: stopDelay,
	}
}

func (m *mockPod) Stop() {
	close(m.stopChan)
	if m.stopDelay > 0 {
		time.AfterFunc(m.stopDelay, func() {
			m.running = false
		})
	} else {
		m.running = false
	}
}

func (m *mockPod) IsRunning() bool {
	return m.running
}

func (m *mockPod) CurrentTaskID() string {
	return m.taskID
}

func TestRecoverFromCrash_NoRunningTasks(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("") // empty webhook for testing

	err := RecoverFromCrash(db, discord)
	if err != nil {
		t.Fatalf("RecoverFromCrash failed: %v", err)
	}

	// Verify no tasks were updated
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status='RETRY'").Scan(&count)
	if err != nil {
		t.Fatalf("query retry tasks: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 retry tasks, got %d", count)
	}
}

func TestRecoverFromCrash_RecoversRunningTasks(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("") // empty webhook for testing

	// Insert tasks with different statuses
	tasks := []struct {
		id     string
		status string
	}{
		{uuid.New().String(), "RUNNING"},
		{uuid.New().String(), "RUNNING"},
		{uuid.New().String(), "PENDING"},
		{uuid.New().String(), "COMPLETED"},
	}

	for _, task := range tasks {
		_, err := db.Exec(`
			INSERT INTO tasks (id, title, type, status)
			VALUES (?, ?, ?, ?)
		`, task.id, "Test Task", "IMPLEMENTATION", task.status)
		if err != nil {
			t.Fatalf("insert task: %v", err)
		}
	}

	// Run recovery
	err := RecoverFromCrash(db, discord)
	if err != nil {
		t.Fatalf("RecoverFromCrash failed: %v", err)
	}

	// Verify only RUNNING tasks were moved to RETRY
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status='RETRY'").Scan(&count)
	if err != nil {
		t.Fatalf("query retry tasks: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 retry tasks, got %d", count)
	}

	// Verify RUNNING tasks no longer exist
	err = db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status='RUNNING'").Scan(&count)
	if err != nil {
		t.Fatalf("query running tasks: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 running tasks after recovery, got %d", count)
	}
}

func TestRecoverFromCrash_SetsCrashRecoveryFlag(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")

	taskID := uuid.New().String()
	_, err := db.Exec(`
		INSERT INTO tasks (id, title, type, status, crash_recovery)
		VALUES (?, ?, ?, ?, ?)
	`, taskID, "Test Task", "IMPLEMENTATION", "RUNNING", false)
	if err != nil {
		t.Fatalf("insert task: %v", err)
	}

	err = RecoverFromCrash(db, discord)
	if err != nil {
		t.Fatalf("RecoverFromCrash failed: %v", err)
	}

	// Verify crash_recovery flag was set
	var crashRecovery bool
	err = db.QueryRow("SELECT crash_recovery FROM tasks WHERE id=?", taskID).Scan(&crashRecovery)
	if err != nil {
		t.Fatalf("query crash_recovery: %v", err)
	}
	if !crashRecovery {
		t.Error("expected crash_recovery=true, got false")
	}
}

func TestGracefulShutdown_PodsStopGracefully(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")

	// Create mock pods that stop quickly
	pods := []Pod{
		newMockPod("task-1", 100*time.Millisecond),
		newMockPod("task-2", 150*time.Millisecond),
	}

	cfg := &config.ShutdownConfig{
		PodGracePeriod: 2 * time.Second,
	}

	ctx := context.Background()
	err := GracefulShutdown(ctx, cfg, pods, db, discord)
	if err != nil {
		t.Fatalf("GracefulShutdown failed: %v", err)
	}

	// Verify all pods stopped
	for i, pod := range pods {
		if pod.IsRunning() {
			t.Errorf("pod %d still running after graceful shutdown", i)
		}
	}
}

func TestGracefulShutdown_ForceKillAfterGracePeriod(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")

	// Insert tasks for the pods
	taskID1 := uuid.New().String()
	taskID2 := uuid.New().String()

	for _, taskID := range []string{taskID1, taskID2} {
		_, err := db.Exec(`
			INSERT INTO tasks (id, title, type, status)
			VALUES (?, ?, ?, ?)
		`, taskID, "Test Task", "IMPLEMENTATION", "RUNNING")
		if err != nil {
			t.Fatalf("insert task: %v", err)
		}
	}

	// Create mock pods that never stop
	pods := []Pod{
		newMockPod(taskID1, 0), // never stops (stopDelay=0 but we don't set running=false)
		newMockPod(taskID2, 0),
	}
	// Make them never actually stop
	for _, pod := range pods {
		mp := pod.(*mockPod)
		mp.stopDelay = 999 * time.Hour // effectively never stops
	}

	cfg := &config.ShutdownConfig{
		PodGracePeriod: 200 * time.Millisecond, // short grace period
	}

	ctx := context.Background()
	err := GracefulShutdown(ctx, cfg, pods, db, discord)
	if err != nil {
		t.Fatalf("GracefulShutdown failed: %v", err)
	}

	// Verify tasks were moved to RETRY with crash_recovery=true
	for _, taskID := range []string{taskID1, taskID2} {
		var status string
		var crashRecovery bool
		err = db.QueryRow(`
			SELECT status, crash_recovery
			FROM tasks
			WHERE id=?
		`, taskID).Scan(&status, &crashRecovery)
		if err != nil {
			t.Fatalf("query task %s: %v", taskID, err)
		}
		if status != "RETRY" {
			t.Errorf("task %s: expected status=RETRY, got %s", taskID, status)
		}
		if !crashRecovery {
			t.Errorf("task %s: expected crash_recovery=true, got false", taskID)
		}
	}
}

func TestGracefulShutdown_NoPodsNoError(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")

	cfg := &config.ShutdownConfig{
		PodGracePeriod: 1 * time.Second,
	}

	ctx := context.Background()
	err := GracefulShutdown(ctx, cfg, []Pod{}, db, discord)
	if err != nil {
		t.Fatalf("GracefulShutdown with no pods failed: %v", err)
	}
}

func TestGracefulShutdown_ContextCanceled(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")

	taskID := uuid.New().String()
	_, err := db.Exec(`
		INSERT INTO tasks (id, title, type, status)
		VALUES (?, ?, ?, ?)
	`, taskID, "Test Task", "IMPLEMENTATION", "RUNNING")
	if err != nil {
		t.Fatalf("insert task: %v", err)
	}

	// Create pod that never stops
	pods := []Pod{
		newMockPod(taskID, 999*time.Hour),
	}

	cfg := &config.ShutdownConfig{
		PodGracePeriod: 10 * time.Second,
	}

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = GracefulShutdown(ctx, cfg, pods, db, discord)
	if err != nil {
		t.Fatalf("GracefulShutdown failed: %v", err)
	}

	// Verify task was force-killed
	var status string
	err = db.QueryRow("SELECT status FROM tasks WHERE id=?", taskID).Scan(&status)
	if err != nil {
		t.Fatalf("query task: %v", err)
	}
	if status != "RETRY" {
		t.Errorf("expected status=RETRY after context cancel, got %s", status)
	}
}

func TestGracefulShutdown_MixedPodTypes(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")

	// Insert tasks
	taskID1 := uuid.New().String()
	taskID2 := uuid.New().String()
	taskID3 := uuid.New().String()

	for _, taskID := range []string{taskID1, taskID2, taskID3} {
		_, err := db.Exec(`
			INSERT INTO tasks (id, title, type, status)
			VALUES (?, ?, ?, ?)
		`, taskID, "Test Task", "IMPLEMENTATION", "RUNNING")
		if err != nil {
			t.Fatalf("insert task: %v", err)
		}
	}

	// Create mix of pods: some that stop gracefully, some that don't
	pods := []Pod{
		newMockPod(taskID1, 100*time.Millisecond), // stops quickly
		newMockPod(taskID2, 50*time.Millisecond),  // stops quickly
		newMockPod(taskID3, 999*time.Hour),        // never stops
	}

	cfg := &config.ShutdownConfig{
		PodGracePeriod: 500 * time.Millisecond,
	}

	ctx := context.Background()
	err := GracefulShutdown(ctx, cfg, pods, db, discord)
	if err != nil {
		t.Fatalf("GracefulShutdown failed: %v", err)
	}

	// Verify first two pods stopped gracefully (tasks still RUNNING)
	for _, taskID := range []string{taskID1, taskID2} {
		var status string
		err = db.QueryRow("SELECT status FROM tasks WHERE id=?", taskID).Scan(&status)
		if err != nil {
			t.Fatalf("query task %s: %v", taskID, err)
		}
		if status != "RUNNING" {
			t.Errorf("task %s: expected status=RUNNING (graceful stop), got %s", taskID, status)
		}
	}

	// Verify third pod was force-killed (task moved to RETRY)
	var status string
	err = db.QueryRow("SELECT status FROM tasks WHERE id=?", taskID3).Scan(&status)
	if err != nil {
		t.Fatalf("query task %s: %v", taskID3, err)
	}
	if status != "RETRY" {
		t.Errorf("task %s: expected status=RETRY (force kill), got %s", taskID3, status)
	}
}
