package server

import "github.com/circle-oo/flux/internal/models"

const defaultTaskPriority = 40

func applyTaskDefaults(task *models.Task, defaultSource string) {
	if task.Priority == 0 {
		task.Priority = defaultTaskPriority
	}
	if task.Source == "" {
		task.Source = defaultSource
	}
}
