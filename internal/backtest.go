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
			// По умолчанию покупаем полностью (для обычных стратегий)
			// Для DCA стратегий частичные покупки обрабатываются через несколько BUY сигналов
			if holdings == 0 && cashCurrent > 0 {
				effectivePrice := price + slippage
				holdings = cashCurrent / effectivePrice
				cashCurrent = 0
				//	fmt.Printf("📈 BUY at %.2f (effective %.2f, candle %d, %s)\n", price, effectivePrice, i, candles[i].Time)
				firstTradeExecuted = true
			} else if holdings > 0 && cashCurrent > 0 {
				// Поддержка частичных покупок для DCA стратегий (усреднение)
				effectivePrice := price + slippage
				// Покупаем на часть доступных средств для усреднения
				buyAmount := cashCurrent * 0.5 // Покупаем на 50% доступных средств
				if buyAmount > 0 {
					newHoldings := buyAmount / effectivePrice
					holdings += newHoldings
					cashCurrent -= buyAmount
					//	fmt.Printf("📈 BUY (averaging) at %.2f (effective %.2f, candle %d, %s)\n", price, effectivePrice, i, candles[i].Time)
				}
			}
		case SELL:
			// КРИТИЧНО: Первая сделка должна быть BUY, игнорируем SELL до первого BUY
			if !firstTradeExecuted {
				continue
			}
			if holdings > 0 {
				effectivePrice := price - slippage
				// По умолчанию продаем полностью (для обычных стратегий)
				// Для DCA стратегий частичные продажи обрабатываются через несколько SELL сигналов
				cashCurrent += holdings * effectivePrice
				holdings = 0
				tradeCount++ // Считаем полную сделку (пару BUY+SELL) только при SELL
				//	fmt.Printf("📉 SELL at %.2f (effective %.2f, candle %d, %s)\n", price, effectivePrice, i, candles[i].Time)
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
