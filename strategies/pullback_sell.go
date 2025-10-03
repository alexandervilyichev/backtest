// strategies/pullback_sell.go

// Pullback Sell Strategy
//
// Описание стратегии:
// Стратегия торговли на откатах (pullbacks) - пытается поймать моменты, когда цена
// после сильного движения делает временный откат, а затем продолжает движение в том же направлении.
// Стратегия покупает при сильном росте и продает при откате, ожидая продолжения тренда.
//
// Как работает:
// - Анализирует процентное изменение цены за определенное количество свечей (чувствительность)
// - Покупка: при значительном росте цены (выше порога buyThreshold)
// - Продажа: при откате цены (падении ниже порога sellThreshold)
// - Предполагается, что после отката цена продолжит движение в первоначальном направлении
//
// Параметры:
// - PullbackSensitivity: количество свечей для анализа движения (обычно 1-3)
//   Чем выше чувствительность, тем больше история движения анализируется
//
// Сильные стороны:
// - Логичная идея следования тренду
// - Хорошо работает в сильных трендовых движениях
// - Простая реализация
// - Может быть эффективна в волатильных рынках
//
// Слабые стороны:
// - Может давать ложные сигналы при развороте тренда
// - Зависит от правильного определения силы движения
// - В боковых рынках генерирует много убыточных сделок
// - Не учитывает общий контекст рынка
//
// Лучшие условия для применения:
// - Сильные трендовые движения
// - Волатильные активы
// - Краткосрочная торговля
// - В сочетании с фильтрами направления тренда

package strategies

import "bt/internal"

type PullbackSellStrategy struct{}

func (s *PullbackSellStrategy) Name() string {
	return "pullback_sell"
}

func (s *PullbackSellStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	// Используем параметр чувствительности для определения силы движения
	sensitivity := params.PullbackSensitivity
	if sensitivity == 0 {
		sensitivity = 1 // значение по умолчанию
	}

	for i := sensitivity; i < len(candles); i++ {
		// Проверяем движение за последние 'sensitivity' свечей
		prevClose := candles[i-sensitivity].Close.ToFloat64()
		currClose := candles[i].Close.ToFloat64()

		// Рассчитываем процентное изменение
		priceChange := (currClose - prevClose) / prevClose * 100

		// BUY: значительный рост цены
		if !inPosition && priceChange > float64(sensitivity)*0.5 { // 0.5% на каждую единицу чувствительности
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		// SELL: откат (падение) цены
		if inPosition && priceChange < -float64(sensitivity)*0.3 { // 0.3% на каждую единицу чувствительности
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *PullbackSellStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{PullbackSensitivity: 1}
	bestProfit := -1.0

	generator := s.GenerateSignals

	for sens := 1; sens <= 3; sens++ {
		params := internal.StrategyParams{PullbackSensitivity: sens}
		signals := generator(candles, params)
		result := internal.Backtest(candles, signals, 0.01) // 0.01 units проскальзывание
		if result.TotalProfit > bestProfit {
			bestProfit = result.TotalProfit
			bestParams = params
		}
	}

	return bestParams
}

func init() {
	internal.RegisterStrategy("pullback_sell", &PullbackSellStrategy{})
}
