# go-musthave-shortener-tpl

Шаблон репозитория для трека «Сервис сокращения URL».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-shortener-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Бенчмарки и профиль памяти

Для анализа производительности добавлены бенчмарки ключевых операций сервиса:

- `BenchmarkGenerateID`
- `BenchmarkShortenerServiceCreate`
- `BenchmarkShortenerServiceCreateBatch`
- `BenchmarkShortenerServiceResolve`

Baseline-профиль памяти сохранён в `profiles/base.pprof`, профиль после оптимизации — в `profiles/result.pprof`.

Команда для снятия профиля:

```bash
go test -run=^$ -bench='Benchmark(GenerateID|ShortenerService)' -benchmem -benchtime=20000x -memprofile=profiles/base.pprof -memprofilerate=1 ./internal/service
```

До оптимизации генерация одного ID использовала `crypto/rand.Int` и `math/big` на каждый символ:

```text
BenchmarkGenerateID-12                     	   20000	     20007 ns/op	     392 B/op	      25 allocs/op
BenchmarkShortenerServiceCreate-12         	   20000	     22040 ns/op	     392 B/op	      25 allocs/op
BenchmarkShortenerServiceCreateBatch-12    	   20000	   2804630 ns/op	   49353 B/op	    2505 allocs/op
BenchmarkShortenerServiceResolve-12        	   20000	         4.225 ns/op	       0 B/op	       0 allocs/op
```

После оптимизации `GenerateID` использует буфер случайных байт и rejection sampling:

```text
BenchmarkGenerateID-12                     	   20000	       921.4 ns/op	       8 B/op	       1 allocs/op
BenchmarkShortenerServiceCreate-12         	   20000	       828.1 ns/op	       8 B/op	       1 allocs/op
BenchmarkShortenerServiceCreateBatch-12    	   20000	     80041 ns/op	   10952 B/op	     105 allocs/op
BenchmarkShortenerServiceResolve-12        	   20000	         3.385 ns/op	       0 B/op	       0 allocs/op
```

Результат сравнения профилей:

```text
$ pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
File: service.test
Type: alloc_space
Time: Jun 17, 2026 at 8:16pm (MSK)
Showing nodes accounting for -747.11MB, 78.09% of 956.68MB total
Dropped 87 nodes (cum <= 4.78MB)
      flat  flat%   sum%        cum   cum%
 -622.59MB 65.08% 65.08%  -747.11MB 78.09%  crypto/rand.Int
 -124.52MB 13.02% 78.09%  -124.52MB 13.02%  math/big.nat.make (inline)
  -15.57MB  1.63% 79.72%   -15.57MB  1.63%  strings.(*Builder).grow
   15.57MB  1.63% 78.09%  -747.11MB 78.09%  github.com/timaogurtzova/shortener/internal/service.GenerateID
         0     0% 78.09%    -7.32MB  0.77%  github.com/timaogurtzova/shortener/internal/service.(*ShortenerService).Create
         0     0% 78.09%  -732.46MB 76.56%  github.com/timaogurtzova/shortener/internal/service.(*ShortenerService).CreateBatch
         0     0% 78.09%  -732.46MB 76.56%  github.com/timaogurtzova/shortener/internal/service.buildBatchRecords
         0     0% 78.09%  -732.46MB 76.56%  github.com/timaogurtzova/shortener/internal/service.generateUniqueID
         0     0% 78.09%    -7.32MB  0.77%  github.com/timaogurtzova/shortener/internal/service_test.BenchmarkGenerateID
         0     0% 78.09%    -7.32MB  0.77%  github.com/timaogurtzova/shortener/internal/service_test.BenchmarkShortenerServiceCreate
         0     0% 78.09%  -732.46MB 76.56%  github.com/timaogurtzova/shortener/internal/service_test.BenchmarkShortenerServiceCreateBatch
         0     0% 78.09%  -124.52MB 13.02%  math/big.(*Int).SetUint64 (inline)
         0     0% 78.09%  -124.52MB 13.02%  math/big.nat.setUint64
         0     0% 78.09%  -124.52MB 13.02%  math/big.nat.setWord (inline)
         0     0% 78.09%   -15.57MB  1.63%  strings.(*Builder).Grow
         0     0% 78.09%  -747.07MB 78.09%  testing.(*B).launch
         0     0% 78.09%  -747.11MB 78.09%  testing.(*B).runN
```
