# 01 Sort

Ручная сортировка в `p95` заменена на `sort.Float64s`.

```powershell
$env:GOCACHE=(Resolve-Path -LiteralPath '.').Path + '\.gocache'
go test -bench Benchmark -benchmem ./internal/stats ./internal/web
go test -bench=BenchmarkCalculateStats_10000 -cpuprofile profiles/01_sort/cpu.pprof ./internal/stats
go test -bench=BenchmarkCalculateStats_10000 -memprofile profiles/01_sort/mem.pprof ./internal/stats
```
