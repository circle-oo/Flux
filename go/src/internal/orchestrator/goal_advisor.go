package orchestrator

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"

	"github.com/circle-oo/flux/internal/models"
	"github.com/circle-oo/flux/internal/notifier"
)

// GoalAdvisor implements SubComponent and sends Discord notifications
// based on heuristics about queue state and goal progress.
type GoalAdvisor struct {
	db      *sql.DB
	discord *notifier.Discord

	mu   sync.Mutex
	sent map[string]bool // dedup: only send each notification once per session
}

// NewGoalAdvisor creates a new GoalAdvisor.
func NewGoalAdvisor(db *sql.DB, discord *notifier.Discord) *GoalAdvisor {
	return &GoalAdvisor{
		db:      db,
		discord: discord,
		sent:    make(map[string]bool),
	}
}

// Name implements SubComponent.
func (g *GoalAdvisor) Name() string {
	return "goal_advisor"
}

// Tick implements SubComponent.
func (g *GoalAdvisor) Tick(_ context.Context) error {
	g.checkEmptyQueue()
	g.checkGoalProgress()
	g.checkOperatorActivity()
	return nil
}

// notify sends a Discord notification if not already sent this session.
func (g *GoalAdvisor) notify(key, msg string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.sent[key] {
		return
	}
	g.sent[key] = true

	slog.Info("goal_advisor: "+msg)
	if g.discord != nil {
		g.discord.Send(notifier.LevelInfo, "Goal Advisor: "+msg)
	}
}

// resetNotification clears a sent flag so it can fire again next time the condition is met.
func (g *GoalAdvisor) resetNotification(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.sent, key)
}

// checkEmptyQueue notifies if no PENDING/READY tasks exist.
func (g *GoalAdvisor) checkEmptyQueue() {
	taskStore := models.NewTaskStore(g.db)

	pending, err := taskStore.CountByStatus(models.TaskPending)
	if err != nil {
		return
	}
	ready, err := taskStore.CountByStatus(models.TaskReady)
	if err != nil {
		return
	}

	if pending+ready > 0 {
		g.resetNotification("empty_queue")
		return
	}

	g.notify("empty_queue", "Task queue is empty. Consider creating new tasks.")
}

// checkGoalProgress notifies when a goal is 80%+ complete.
func (g *GoalAdvisor) checkGoalProgress() {
	goalStore := models.NewGoalStore(g.db)

	goal, err := goalStore.GetCurrent()
	if err != nil || goal == nil {
		return
	}

	var total, completed int
	err = g.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN status = 'COMPLETED' THEN 1 ELSE 0 END), 0)
		 FROM tasks WHERE goal_id = ?`,
		goal.ID,
	).Scan(&total, &completed)
	if err != nil || total == 0 {
		return
	}

	ratio := float64(completed) / float64(total)
	if ratio < 0.8 {
		g.resetNotification("goal_progress")
		return
	}

	g.notify("goal_progress",
		fmt.Sprintf("Goal \"%s\" is %.0f%% complete. Consider planning the next goal.", goal.Title, ratio*100))
}

// checkOperatorActivity notifies if no OPERATOR tasks in 48 hours.
func (g *GoalAdvisor) checkOperatorActivity() {
	var count int
	err := g.db.QueryRow(
		`SELECT COUNT(*) FROM tasks WHERE source = ? AND created_at > datetime('now', '-48 hours')`,
		models.TaskSourceOperator,
	).Scan(&count)
	if err != nil {
		return
	}

	if count > 0 {
		g.resetNotification("operator_checkin")
		return
	}

	g.notify("operator_checkin", "No operator activity in 48h. Check-in recommended.")
}
