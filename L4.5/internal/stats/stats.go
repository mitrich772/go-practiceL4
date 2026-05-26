package stats

import (
	"errors"
	"math"
	"sort"
)

var ErrEmptyInput = errors.New("numbers must not be empty")

type Result struct {
	Count int     `json:"count"`
	Sum   float64 `json:"sum"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Avg   float64 `json:"avg"`
	P95   float64 `json:"p95"`
}

func Calculate(numbers []float64) (Result, error) {
	if len(numbers) == 0 {
		return Result{}, ErrEmptyInput
	}

	total := 0.0
	minValue := numbers[0]
	maxValue := numbers[0]
	for _, value := range numbers {
		total += value
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}

	return Result{
		Count: len(numbers),
		Sum:   total,
		Min:   minValue,
		Max:   maxValue,
		Avg:   total / float64(len(numbers)),
		P95:   percentile95(numbers),
	}, nil
}

func percentile95(numbers []float64) float64 {
	values := clone(numbers)
	sort.Float64s(values)

	index := int(math.Ceil(float64(len(values))*0.95)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func clone(numbers []float64) []float64 {
	values := make([]float64, len(numbers))
	copy(values, numbers)
	return values
}
