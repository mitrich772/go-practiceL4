package stats

import (
	"errors"
	"testing"
)

func TestCalculate(t *testing.T) {
	tests := []struct {
		name    string
		input   []float64
		want    Result
		wantErr error
	}{
		{
			name:    "empty",
			input:   nil,
			wantErr: ErrEmptyInput,
		},
		{
			name:  "single",
			input: []float64{7},
			want:  Result{Count: 1, Sum: 7, Min: 7, Max: 7, Avg: 7, P95: 7},
		},
		{
			name:  "positive and negative",
			input: []float64{-5, 10, 0, 15},
			want:  Result{Count: 4, Sum: 20, Min: -5, Max: 15, Avg: 5, P95: 15},
		},
		{
			name:  "p95 rounded up",
			input: []float64{1, 2, 3, 4, 5},
			want:  Result{Count: 5, Sum: 15, Min: 1, Max: 5, Avg: 3, P95: 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Calculate(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Calculate() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("Calculate() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
