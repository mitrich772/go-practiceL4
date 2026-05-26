# 03 Final

Порог для parallel path поднят до крупных массивов, а `p95` считается через quickselect.

```powershell
$env:GOCACHE=(Resolve-Path -LiteralPath '.').Path + '\.gocache'
go test -bench Benchmark -benchmem ./internal/stats ./internal/web
go test -bench=BenchmarkCalculateStats_10000 -cpuprofile profiles/03_final/cpu.pprof ./internal/stats
go test -bench=BenchmarkCalculateStats_10000 -memprofile profiles/03_final/mem.pprof ./internal/stats
go test -run TestTrace -trace profiles/03_final/trace.out ./internal/stats
```
