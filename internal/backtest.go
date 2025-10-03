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

	cash := 10000.0
	holdings := 0.0
	portfolioValues := []float64{cash}
	tradeCount := 0

	for i, signal := range signals {
		price := candles[i].Close.ToFloat64()

		switch signal {
		case BUY:
			if holdings == 0 && cash > 0 {
				effectivePrice := price + slippage
				holdings = cash / effectivePrice
				cash = 0
				//	fmt.Printf("📈 BUY at %.2f (effective %.2f, candle %d, %s)\n", price, effectivePrice, i, candles[i].Time)
				tradeCount++
			}
		case SELL:
			if holdings > 0 {
				effectivePrice := price - slippage
				cash = holdings * effectivePrice
				holdings = 0
				//	fmt.Printf("📉 SELL at %.2f (effective %.2f, candle %d, %s)\n", price, effectivePrice, i, candles[i].Time)
				tradeCount++
			}
		}

		portfolioValue := cash + holdings*price
		portfolioValues = append(portfolioValues, portfolioValue)
	}

	finalPrice := candles[len(candles)-1].Close.ToFloat64()
	finalPortfolio := cash + holdings*finalPrice
	profit := (finalPortfolio - 10000.0) / 10000.0

	return BacktestResult{
		TotalProfit:     profit,
		TradeCount:      tradeCount,
		FinalPortfolio:  finalPortfolio,
		PortfolioValues: portfolioValues,
	}
}
