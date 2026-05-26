package stats

import (
	"errors"
	"math"
	"runtime"
	"sort"
	"sync"
)

var ErrEmptyInput = errors.New("numbers must not be empty")

const parallelThreshold = 10000

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
	if len(numbers) >= parallelThreshold {
		return calculateParallel(numbers), nil
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

type partialResult struct {
	sum float64
	min float64
	max float64
}

func calculateParallel(numbers []float64) Result {
	workers := runtime.GOMAXPROCS(0)
	if workers > len(numbers) {
		workers = len(numbers)
	}

	partials := make([]partialResult, workers)
	chunkSize := (len(numbers) + workers - 1) / workers
	var wg sync.WaitGroup

	for worker := 0; worker < workers; worker++ {
		start := worker * chunkSize
		end := start + chunkSize
		if end > len(numbers) {
			end = len(numbers)
		}
		if start >= end {
			break
		}

		wg.Add(1)
		go func(index, start, end int) {
			defer wg.Done()
			sum := 0.0
			minValue := numbers[start]
			maxValue := numbers[start]
			for _, value := range numbers[start:end] {
				sum += value
				if value < minValue {
					minValue = value
				}
				if value > maxValue {
					maxValue = value
				}
			}
			partials[index] = partialResult{sum: sum, min: minValue, max: maxValue}
		}(worker, start, end)
	}

	wg.Wait()

	total := 0.0
	minValue := partials[0].min
	maxValue := partials[0].max
	for _, partial := range partials {
		total += partial.sum
		if partial.min < minValue {
			minValue = partial.min
		}
		if partial.max > maxValue {
			maxValue = partial.max
		}
	}

	return Result{
		Count: len(numbers),
		Sum:   total,
		Min:   minValue,
		Max:   maxValue,
		Avg:   total / float64(len(numbers)),
		P95:   percentile95(numbers),
	}
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
