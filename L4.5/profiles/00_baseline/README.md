# 00 Baseline

Первая версия: `map[string]any` в HTTP-слое и ручная сортировка для `p95`.

```powershell
$env:GOCACHE=(Resolve-Path -LiteralPath '.').Path + '\.gocache'
go test -bench Benchmark -benchmem ./internal/stats ./internal/web
go test -bench=BenchmarkCalculateStats_1000 -cpuprofile profiles/00_baseline/cpu.pprof ./internal/stats
go test -bench=BenchmarkCalculateStats_1000 -memprofile profiles/00_baseline/mem.pprof ./internal/stats
go test -run TestTrace -trace profiles/00_baseline/trace.out ./internal/stats
```
