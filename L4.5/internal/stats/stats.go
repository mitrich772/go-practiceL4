package stats

import (
	"errors"
	"math"
	"runtime"
	"sync"
)

var ErrEmptyInput = errors.New("numbers must not be empty")

const parallelThreshold = 50000

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
	return quickselect(values, percentileIndex(len(values)))
}

func percentileIndex(length int) int {
	index := int(math.Ceil(float64(length)*0.95)) - 1
	if index < 0 {
		index = 0
	}
	if index >= length {
		index = length - 1
	}
	return index
}

func clone(numbers []float64) []float64 {
	values := make([]float64, len(numbers))
	copy(values, numbers)
	return values
}

func quickselect(values []float64, target int) float64 {
	left := 0
	right := len(values) - 1

	for left < right {
		pivot := partition(values, left, right)
		switch {
		case pivot == target:
			return values[pivot]
		case pivot < target:
			left = pivot + 1
		default:
			right = pivot - 1
		}
	}

	return values[left]
}

func partition(values []float64, left, right int) int {
	pivotIndex := medianOfThree(values, left, right)
	values[pivotIndex], values[right] = values[right], values[pivotIndex]
	pivotValue := values[right]
	store := left

	for i := left; i < right; i++ {
		if values[i] < pivotValue {
			values[store], values[i] = values[i], values[store]
			store++
		}
	}

	values[right], values[store] = values[store], values[right]
	return store
}

func medianOfThree(values []float64, left, right int) int {
	middle := left + (right-left)/2

	if values[left] > values[middle] {
		values[left], values[middle] = values[middle], values[left]
	}
	if values[left] > values[right] {
		values[left], values[right] = values[right], values[left]
	}
	if values[middle] > values[right] {
		values[middle], values[right] = values[right], values[middle]
	}

	return middle
}
