// backtest.go — исправленная, надёжная версия
package internal

import (
	"log"
)

type BacktestResult struct {
	TotalProfit     float64
	TradeCount      int
	FinalPortfolio  float64
	PortfolioValues []float64
}

func Backtest(candles []Candle, signals []SignalType, slippage float64) BacktestResult {

	if len(candles) != len(signals) {
		log.Fatal("Mismatch between candles and signals length")
	}

	cashCurrent, initCash := 10000.0, 10000.0
	holdings := 0.0
	portfolioValues := []float64{cashCurrent}
	tradeCount := 0
	firstTradeExecuted := false // Флаг для отслеживания первой сделки

	for i, signal := range signals {
		price := candles[i].Close.ToFloat64()

		switch signal {
		case BUY:
			if holdings == 0 && cashCurrent > 0 {
				effectivePrice := price + slippage
				holdings = cashCurrent / effectivePrice
				cashCurrent = 0
				//	fmt.Printf("📈 BUY at %.2f (effective %.2f, candle %d, %s)\n", price, effectivePrice, i, candles[i].Time)
				firstTradeExecuted = true
			}
		case SELL:
			// КРИТИЧНО: Первая сделка должна быть BUY, игнорируем SELL до первого BUY
			if !firstTradeExecuted {
				continue
			}
			if holdings > 0 {
				effectivePrice := price - slippage
				cashCurrent = holdings * effectivePrice
				holdings = 0
				//	fmt.Printf("📉 SELL at %.2f (effective %.2f, candle %d, %s)\n", price, effectivePrice, i, candles[i].Time)
				tradeCount++ // Считаем полную сделку (пару BUY+SELL) только при SELL
			}
		}

		portfolioValue := cashCurrent + holdings*price
		portfolioValues = append(portfolioValues, portfolioValue)
	}

	finalPrice := candles[len(candles)-1].Close.ToFloat64()
	finalPortfolio := cashCurrent + holdings*finalPrice
	profit := (finalPortfolio - initCash) / initCash

	return BacktestResult{
		TotalProfit:     profit,
		TradeCount:      tradeCount,
		FinalPortfolio:  finalPortfolio,
		PortfolioValues: portfolioValues,
	}
}
