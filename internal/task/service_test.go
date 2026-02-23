package task

import "testing"

func TestIsTerminal_Statuses(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: "task_postprocess_end", want: true},
		{status: "task_cancel", want: true},
		{status: "task_error_full", want: true},
		{status: "task_end", want: false},
		{status: "task_start", want: false},
		{status: "", want: false},
	}

	for _, tc := range tests {
		if got := isTerminal(tc.status); got != tc.want {
			t.Fatalf("isTerminal(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}
