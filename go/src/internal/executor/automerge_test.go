package executor

import (
	"strings"
	"testing"

	"github.com/circle-oo/flux/internal/models"
)

func TestAutoMergeReason(t *testing.T) {
	tests := []struct {
		name           string
		task           *models.Task
		diffLines      int
		filesChanged   int
		wantMerge      bool
		wantReasonHas  string
	}{
		{
			name:          "large diff lines blocks auto-merge",
			task:          &models.Task{Source: models.TaskSourceSystem, Type: models.TaskTypeCoding},
			diffLines:     2500,
			filesChanged:  5,
			wantMerge:     false,
			wantReasonHas: "diff too large",
		},
		{
			name:          "too many files blocks auto-merge",
			task:          &models.Task{Source: models.TaskSourceSystem, Type: models.TaskTypeCoding},
			diffLines:     100,
			filesChanged:  25,
			wantMerge:     false,
			wantReasonHas: "too many files",
		},
		{
			name:          "large diff and many files shows both reasons",
			task:          &models.Task{Source: models.TaskSourceSystem, Type: models.TaskTypeCoding},
			diffLines:     3000,
			filesChanged:  30,
			wantMerge:     false,
			wantReasonHas: "diff too large",
		},
		{
			name:          "system source auto-merges",
			task:          &models.Task{Source: models.TaskSourceSystem, Type: models.TaskTypeCoding},
			diffLines:     50,
			filesChanged:  2,
			wantMerge:     true,
			wantReasonHas: "source is system",
		},
		{
			name:          "self source auto-merges",
			task:          &models.Task{Source: models.TaskSourceSelf, Type: models.TaskTypeCoding},
			diffLines:     50,
			filesChanged:  2,
			wantMerge:     true,
			wantReasonHas: "source is self",
		},
		{
			name:          "maintenance type auto-merges",
			task:          &models.Task{Source: models.TaskSourceOperator, Type: models.TaskTypeMaintenance},
			diffLines:     200,
			filesChanged:  5,
			wantMerge:     true,
			wantReasonHas: "maintenance",
		},
		{
			name:          "high-priority bugfix auto-merges",
			task:          &models.Task{Source: models.TaskSourceOperator, Type: models.TaskTypeBugfix, Priority: 5},
			diffLines:     200,
			filesChanged:  5,
			wantMerge:     true,
			wantReasonHas: "bugfix",
		},
		{
			name:          "low-priority bugfix requires review",
			task:          &models.Task{Source: models.TaskSourceOperator, Type: models.TaskTypeBugfix, Priority: 20},
			diffLines:     200,
			filesChanged:  5,
			wantMerge:     false,
			wantReasonHas: "Requires operator review",
		},
		{
			name:          "small change auto-merges",
			task:          &models.Task{Source: models.TaskSourceOperator, Type: models.TaskTypeCoding},
			diffLines:     50,
			filesChanged:  2,
			wantMerge:     true,
			wantReasonHas: "small change",
		},
		{
			name:          "large operator coding task requires review",
			task:          &models.Task{Source: models.TaskSourceOperator, Type: models.TaskTypeCoding},
			diffLines:     500,
			filesChanged:  10,
			wantMerge:     false,
			wantReasonHas: "Requires operator review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMerge, gotReason := AutoMergeReason(tt.task, tt.diffLines, tt.filesChanged)
			if gotMerge != tt.wantMerge {
				t.Errorf("AutoMergeReason() merge = %v, want %v", gotMerge, tt.wantMerge)
			}
			if !strings.Contains(strings.ToLower(gotReason), strings.ToLower(tt.wantReasonHas)) {
				t.Errorf("AutoMergeReason() reason = %q, want it to contain %q", gotReason, tt.wantReasonHas)
			}
		})
	}
}

func TestAutoMergeReasonConsistentWithShouldAutoMerge(t *testing.T) {
	// Verify AutoMergeReason and ShouldAutoMerge always agree
	cases := []struct {
		task         *models.Task
		diffLines    int
		filesChanged int
	}{
		{&models.Task{Source: models.TaskSourceSystem, Type: models.TaskTypeCoding}, 50, 2},
		{&models.Task{Source: models.TaskSourceSelf, Type: models.TaskTypeCoding}, 50, 2},
		{&models.Task{Source: models.TaskSourceOperator, Type: models.TaskTypeMaintenance}, 200, 5},
		{&models.Task{Source: models.TaskSourceOperator, Type: models.TaskTypeBugfix, Priority: 5}, 200, 5},
		{&models.Task{Source: models.TaskSourceOperator, Type: models.TaskTypeBugfix, Priority: 20}, 200, 5},
		{&models.Task{Source: models.TaskSourceOperator, Type: models.TaskTypeCoding}, 50, 2},
		{&models.Task{Source: models.TaskSourceOperator, Type: models.TaskTypeCoding}, 500, 10},
		{&models.Task{Source: models.TaskSourceSystem, Type: models.TaskTypeCoding}, 2500, 5},
		{&models.Task{Source: models.TaskSourceSystem, Type: models.TaskTypeCoding}, 100, 25},
	}

	for _, c := range cases {
		shouldMerge := ShouldAutoMerge(c.task, c.diffLines, c.filesChanged)
		reasonMerge, _ := AutoMergeReason(c.task, c.diffLines, c.filesChanged)
		if shouldMerge != reasonMerge {
			t.Errorf("ShouldAutoMerge=%v but AutoMergeReason=%v for task source=%s type=%s diff=%d files=%d",
				shouldMerge, reasonMerge, c.task.Source, c.task.Type, c.diffLines, c.filesChanged)
		}
	}
}
