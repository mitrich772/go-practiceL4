package stats

import "testing"

func BenchmarkCalculateStats_100(b *testing.B) {
	benchmarkCalculateStats(b, 100)
}

func BenchmarkCalculateStats_1000(b *testing.B) {
	benchmarkCalculateStats(b, 1000)
}

func BenchmarkCalculateStats_10000(b *testing.B) {
	benchmarkCalculateStats(b, 10000)
}

func TestTrace(t *testing.T) {
	numbers := makeNumbers(1000)
	for i := 0; i < 200; i++ {
		if _, err := Calculate(numbers); err != nil {
			t.Fatal(err)
		}
	}
}

func benchmarkCalculateStats(b *testing.B, size int) {
	numbers := makeNumbers(size)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result, err := Calculate(numbers)
		if err != nil {
			b.Fatal(err)
		}
		sinkResult = result
	}
}

func makeNumbers(size int) []float64 {
	numbers := make([]float64, size)
	for i := range numbers {
		numbers[i] = float64((i * 37) % 1000)
	}
	return numbers
}

var sinkResult Result
