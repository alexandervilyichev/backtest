package internal

import (
	"testing"
)

func TestBacktest_FirstTradeMustBeBuy(t *testing.T) {
	// Создаем тестовые свечи
	candles := []Candle{
		{Close: Price(100.0)},
		{Close: Price(105.0)},
		{Close: Price(110.0)},
		{Close: Price(108.0)},
		{Close: Price(112.0)},
	}

	// Тест 1: Первый сигнал SELL - должен быть проигнорирован
	signals := []SignalType{SELL, BUY, HOLD, SELL, HOLD}
	result := Backtest(candles, signals, 0.0)

	// Должна быть 1 сделка (BUY на индексе 1 + SELL на индексе 3)
	if result.TradeCount != 1 {
		t.Errorf("Expected 1 trade, got %d (first SELL should be ignored)", result.TradeCount)
	}

	// Тест 2: Первый сигнал BUY - должен быть выполнен
	signals2 := []SignalType{BUY, HOLD, SELL, HOLD, HOLD}
	result2 := Backtest(candles, signals2, 0.0)

	if result2.TradeCount != 1 {
		t.Errorf("Expected 1 trade, got %d", result2.TradeCount)
	}
}

func TestBacktest_TradeCountIsPairs(t *testing.T) {
	candles := []Candle{
		{Close: Price(100.0)},
		{Close: Price(105.0)},
		{Close: Price(110.0)},
		{Close: Price(108.0)},
		{Close: Price(112.0)},
		{Close: Price(115.0)},
	}

	// BUY-SELL-BUY-SELL = 2 полные сделки
	signals := []SignalType{BUY, HOLD, SELL, BUY, HOLD, SELL}
	result := Backtest(candles, signals, 0.0)

	if result.TradeCount != 2 {
		t.Errorf("Expected 2 trades (2 BUY+SELL pairs), got %d", result.TradeCount)
	}

	// BUY-SELL-BUY (незакрытая) = 1 полная сделка
	signals2 := []SignalType{BUY, HOLD, SELL, BUY, HOLD, HOLD}
	result2 := Backtest(candles, signals2, 0.0)

	if result2.TradeCount != 1 {
		t.Errorf("Expected 1 trade (only completed pairs count), got %d", result2.TradeCount)
	}
}

func TestBacktest_SignalAlternation(t *testing.T) {
	candles := []Candle{
		{Close: Price(100.0)},
		{Close: Price(105.0)},
		{Close: Price(110.0)},
		{Close: Price(108.0)},
		{Close: Price(112.0)},
	}

	// Два BUY подряд - второй должен быть проигнорирован
	signals := []SignalType{BUY, BUY, SELL, HOLD, HOLD}
	result := Backtest(candles, signals, 0.0)

	if result.TradeCount != 1 {
		t.Errorf("Expected 1 trade (second BUY should be ignored), got %d", result.TradeCount)
	}

	// Два SELL подряд - второй должен быть проигнорирован
	signals2 := []SignalType{BUY, HOLD, SELL, SELL, HOLD}
	result2 := Backtest(candles, signals2, 0.0)

	if result2.TradeCount != 1 {
		t.Errorf("Expected 1 trade (second SELL should be ignored), got %d", result2.TradeCount)
	}
}
