# L4.5 Stats API Profiling

HTTP API для расчета статистики по массиву чисел. Сервис принимает JSON, считает `count`, `sum`, `min`, `max`, `avg`, `p95` и отдает JSON-ответ.

В этой работе важна не сама математика, а история замеров: сначала рабочая первая версия, потом benchmark, pprof, trace и несколько правок по результатам.

## API

Запуск:

```powershell
go run ./cmd/stats-api -addr localhost:8080
```

Проверка:

```powershell
curl.exe http://localhost:8080/health
curl.exe -X POST http://localhost:8080/stats -H "Content-Type: application/json" -d "{\"numbers\":[1,2,3,4,5]}"
```

Ответ:

```json
{"count":5,"sum":15,"min":1,"max":5,"avg":3,"p95":5}
```

pprof endpoints:

```text
http://localhost:8080/debug/pprof/
http://localhost:8080/debug/pprof/profile
http://localhost:8080/debug/pprof/heap
http://localhost:8080/debug/pprof/trace
```

Нагрузка:

```powershell
go run ./cmd/loadgen -url http://localhost:8080/stats -requests 10000 -concurrency 16 -size 1000
```

## Коммиты

```text
feat(l4.5): add realistic baseline stats api
test(l4.5): add benchmarks and baseline profiles
perf(l4.5): replace naive percentile sort
perf(l4.5): add parallel aggregation for large inputs
perf(l4.5): tune threshold and percentile selection
docs(l4.5): document profiling results
```

## Что менялось

Первая версия была обычной рабочей реализацией, но с двумя спорными местами:

- HTTP-слой разбирал request через `map[string]any` и руками переводил `[]any` в `[]float64`.
- `p95` считался через копию массива и простую ручную сортировку.

`sum`, `min`, `max`, `avg` с самого начала считались за один проход. Лишнего копирования массива под каждую метрику нет.

После замеров ручная сортировка стала главным узким местом. Ее заменил `sort.Float64s`, и на `10000` чисел время упало с `43386727 ns/op` до `371343 ns/op`.

Потом была добавлена параллельная агрегация для больших входов: массив режется на чанки, goroutines считают локальные `sum`, `min`, `max`, затем результаты объединяются. Этот шаг полезен как проверка гипотезы: на `10000` чисел стало хуже (`371343 ns/op` -> `413531 ns/op`), потому что overhead goroutines оказался больше выигрыша.

В финальной версии threshold поднят до `50000`, поэтому средние массивы остаются на последовательном пути. Для `p95` полная сортировка заменена на quickselect по одной копии массива. Это дало основной финальный выигрыш: `BenchmarkCalculateStats_10000` дошел до `29285 ns/op`.

## Benchmarks

Команда:

```powershell
$env:GOCACHE=(Resolve-Path -LiteralPath '.').Path + '\.gocache'
go test -bench Benchmark -benchmem ./internal/stats ./internal/web
```

| Benchmark | Baseline | Sort | Parallel | Final |
| --- | ---: | ---: | ---: | ---: |
| `CalculateStats_100` | 4511 ns/op | 1085 ns/op | 1064 ns/op | 362.8 ns/op |
| `CalculateStats_1000` | 538672 ns/op | 16408 ns/op | 15422 ns/op | 2995 ns/op |
| `CalculateStats_10000` | 43386727 ns/op | 371343 ns/op | 413531 ns/op | 29285 ns/op |
| `StatsHandler_1000` | 89157 ns/op | 85442 ns/op | 85191 ns/op | 85556 ns/op |
| `StatsHandler_10000` | 59553168 ns/op | 1074360 ns/op | 1110769 ns/op | 1198890 ns/op |

Память у `CalculateStats_10000`:

```text
baseline: 85070 B/op, 1 alloc/op
sort:     81945 B/op, 1 alloc/op
parallel: 83629 B/op, 27 allocs/op
final:    81922 B/op, 1 alloc/op
```

Параллельная версия отдельно показала цену goroutines: больше allocations и нет выигрыша на этом размере входа. Поэтому в финале parallel path оставлен только для крупных массивов.

## Профили

Снимки лежат в `profiles`:

- `00_baseline` - первая версия с ручной сортировкой.
- `01_sort` - замена сортировки.
- `02_parallel` - первая попытка с goroutines.
- `03_final` - threshold и quickselect.

CPU profile:

```powershell
go test -bench=BenchmarkCalculateStats_10000 -cpuprofile profiles/03_final/cpu.pprof ./internal/stats
go tool pprof profiles/03_final/cpu.pprof
```

Memory profile:

```powershell
go test -bench=BenchmarkCalculateStats_10000 -memprofile profiles/03_final/mem.pprof ./internal/stats
go tool pprof profiles/03_final/mem.pprof
```

Trace:

```powershell
go test -run TestTrace -trace profiles/03_final/trace.out ./internal/stats
go tool trace profiles/03_final/trace.out
```

Сравнение через benchstat:

```powershell
go install golang.org/x/perf/cmd/benchstat@latest
benchstat profiles/00_baseline/bench.txt profiles/03_final/bench.txt
```

## Вывод

Самая заметная ошибка первой версии была не в HTTP и не в одном проходе по массиву, а в алгоритме percentile. Ручная сортировка быстро ломает время на больших payload.

Распараллеливание само по себе не гарантирует ускорение. На средних массивах оно добавило работу планировщику и allocations. Поэтому финальная версия использует threshold и не запускает goroutines там, где последовательный путь быстрее.
