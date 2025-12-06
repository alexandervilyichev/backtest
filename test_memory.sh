#!/bin/bash

# Скрипт для тестирования потребления памяти с разным количеством воркеров

echo "🧪 Тестирование потребления памяти backtester"
echo "=============================================="
echo ""

FILE="tmos_big.json"

if [ ! -f "$FILE" ]; then
    echo "❌ Файл $FILE не найден"
    exit 1
fi

echo "📊 Файл: $FILE"
CANDLES=$(jq '.candles | length' "$FILE")
echo "📈 Свечей: $CANDLES"
echo ""

# Тест 1: Все CPU (по умолчанию)
echo "🔥 Тест 1: Автоматический режим (все CPU)"
echo "Команда: go run ./cmd/backtester/ -file=$FILE -strategy=all -mem_profile=mem_auto.prof"
/usr/bin/time -v go run ./cmd/backtester/ -file="$FILE" -strategy=all -mem_profile=mem_auto.prof 2>&1 | grep -E "(Maximum resident|User time|System time)"
echo ""

# Тест 2: 4 воркера
echo "🔥 Тест 2: 4 воркера"
echo "Команда: go run ./cmd/backtester/ -file=$FILE -strategy=all -workers=4 -mem_profile=mem_4workers.prof"
/usr/bin/time -v go run ./cmd/backtester/ -file="$FILE" -strategy=all -workers=4 -mem_profile=mem_4workers.prof 2>&1 | grep -E "(Maximum resident|User time|System time)"
echo ""

# Тест 3: 2 воркера
echo "🔥 Тест 3: 2 воркера"
echo "Команда: go run ./cmd/backtester/ -file=$FILE -strategy=all -workers=2 -mem_profile=mem_2workers.prof"
/usr/bin/time -v go run ./cmd/backtester/ -file="$FILE" -strategy=all -workers=2 -mem_profile=mem_2workers.prof 2>&1 | grep -E "(Maximum resident|User time|System time)"
echo ""

# Тест 4: 1 воркер
echo "🔥 Тест 4: 1 воркер (последовательное выполнение)"
echo "Команда: go run ./cmd/backtester/ -file=$FILE -strategy=all -workers=1 -mem_profile=mem_1worker.prof"
/usr/bin/time -v go run ./cmd/backtester/ -file="$FILE" -strategy=all -workers=1 -mem_profile=mem_1worker.prof 2>&1 | grep -E "(Maximum resident|User time|System time)"
echo ""

echo "✅ Тестирование завершено"
echo ""
echo "📊 Анализ профилей памяти:"
echo "  go tool pprof -http=:8080 mem_auto.prof"
echo "  go tool pprof -http=:8081 mem_4workers.prof"
echo "  go tool pprof -http=:8082 mem_2workers.prof"
echo "  go tool pprof -http=:8083 mem_1worker.prof"
