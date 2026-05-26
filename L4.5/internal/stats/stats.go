package stats

import (
	"errors"
	"math"
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
	bubbleSort(values)

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

func bubbleSort(values []float64) {
	for i := 0; i < len(values); i++ {
		swapped := false
		for j := 1; j < len(values)-i; j++ {
			if values[j-1] > values[j] {
				values[j-1], values[j] = values[j], values[j-1]
				swapped = true
			}
		}
		if !swapped {
			return
		}
	}
}
