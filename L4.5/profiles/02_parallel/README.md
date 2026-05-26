# 02 Parallel

Для больших массивов сумма, минимум и максимум считаются по чанкам в goroutines.

```powershell
$env:GOCACHE=(Resolve-Path -LiteralPath '.').Path + '\.gocache'
go test -bench Benchmark -benchmem ./internal/stats ./internal/web
go test -bench=BenchmarkCalculateStats_10000 -cpuprofile profiles/02_parallel/cpu.pprof ./internal/stats
go test -bench=BenchmarkCalculateStats_10000 -memprofile profiles/02_parallel/mem.pprof ./internal/stats
```
