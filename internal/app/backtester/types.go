package backtester

import (
	"bt/internal"
	"time"
)

// BenchmarkResult — результат тестирования стратегии
type BenchmarkResult struct {
	Name           string
	TotalProfit    float64
	TradeCount     int
	FinalPortfolio float64
	ExecutionTime  time.Duration
}

// CandleWithSignal — свеча с сигналом для построения графиков
type CandleWithSignal struct {
	Time   string              `json:"time"`
	Open   float64             `json:"open"`
	High   float64             `json:"high"`
	Low    float64             `json:"low"`
	Close  float64             `json:"close"`
	Volume float64             `json:"volume"`
	Signal internal.SignalType `json:"signal"`
}

// StrategyRunner — интерфейс для запуска стратегий
type StrategyRunner interface {
	RunStrategy(strategyName string, candles []internal.Candle) (*BenchmarkResult, error)
	RunAllStrategies(candles []internal.Candle) ([]BenchmarkResult, error)
}

// ResultSaver — интерфейс для сохранения результатов
type ResultSaver interface {
	SaveTopStrategies(candles []internal.Candle, results []BenchmarkResult, inputFilename string, topN int) error
}

// ResultPrinter — интерфейс для вывода результатов
type ResultPrinter interface {
	PrintComparison(results []BenchmarkResult)
	PrintProgress(current, total int)
}

// Config — конфигурация приложения
type Config struct {
	Filename    string
	Strategy    string
	Debug       bool
	SaveSignals int
}
