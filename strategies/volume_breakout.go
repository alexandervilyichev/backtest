// strategies/volume_breakout.go

// Volume Breakout Strategy
//
// Описание стратегии:
// Стратегия основана на анализе объема торгов как подтверждения силы ценового движения.
// Высокий объем при ценовом движении указывает на сильный интерес участников рынка,
// что может сигнализировать о продолжении движения. Стратегия ищет breakout моменты
// с повышенным объемом как сигналы для входа в позицию.
//
// Как работает:
// - Рассчитывается средний объем за последние 3 свечи
// - Покупка: зеленая свеча (close > open) с объемом выше среднего в volumeMultiplier раз
// - Продажа: красная свеча (close < open) или объем в 2*volumeMultiplier раз выше среднего
// - Объем подтверждает силу движения: высокий объем = сильный интерес = продолжение движения
//
// Параметры:
// - VolumeMultiplier: множитель для определения высокого объема (обычно 1.2-2.0)
//   Чем выше множитель, тем более значимый объем требуется для сигнала
//
// Сильные стороны:
// - Учитывает рыночную активность через объем
// - Хорошо фильтрует слабые движения
// - Логичная идея: объем подтверждает силу тренда
// - Может быть эффективна в различных рыночных условиях
//
// Слабые стороны:
// - Зависит от качества данных объема
// - Может пропускать движения без высокого объема
// - В волатильных условиях может генерировать ложные сигналы
// - Требует достаточной истории для расчета среднего объема
//
// Лучшие условия для применения:
// - Рынки с хорошей ликвидностью и объемом данных
// - В сочетании с ценовым анализом
// - На активах с понятной динамикой объема
// - Среднесрочная торговля с подтверждением тренда

package strategies

import (
	"bt/internal"
	"log"
	"strconv"
)

type VolumeBreakoutStrategy struct{}

func (s *VolumeBreakoutStrategy) Name() string {
	return "volume_breakout"
}

func (s *VolumeBreakoutStrategy) GenerateSignals(candles []internal.Candle, params internal.StrategyParams) []internal.SignalType {
	signals := make([]internal.SignalType, len(candles))
	inPosition := false

	// Устанавливаем значение по умолчанию для VolumeMultiplier
	volumeMultiplier := params.VolumeMultiplier
	if volumeMultiplier == 0 {
		volumeMultiplier = 1.5 // разумное значение по умолчанию
	}

	for i := range candles {
		if i < 3 {
			signals[i] = internal.HOLD
			continue
		}

		var totalVolume float64
		for j := i - 3; j < i; j++ {
			vol, err := strconv.ParseFloat(candles[j].Volume, 64)
			if err != nil {
				log.Printf("Предупреждение: некорректный объем на свече %d: %s, используем 0", j, candles[j].Volume)
				vol = 0
			}
			totalVolume += vol
		}
		avgVolume := totalVolume / 3.0

		currentVol, err := strconv.ParseFloat(candles[i].Volume, 64)
		if err != nil {
			log.Printf("Предупреждение: некорректный объем на свече %d: %s, используем 0", i, candles[i].Volume)
			currentVol = 0
		}

		openPrice := candles[i].Open.ToFloat64()
		closePrice := candles[i].Close.ToFloat64()

		// BUY: зеленая свеча с высоким объемом
		if !inPosition && closePrice > openPrice && currentVol > avgVolume*volumeMultiplier {
			signals[i] = internal.BUY
			inPosition = true
			continue
		}

		// SELL: красная свеча или достижение цели
		if inPosition && (closePrice < openPrice || currentVol > avgVolume*volumeMultiplier*2) {
			signals[i] = internal.SELL
			inPosition = false
			continue
		}

		signals[i] = internal.HOLD
	}

	return signals
}

func (s *VolumeBreakoutStrategy) Optimize(candles []internal.Candle) internal.StrategyParams {
	bestParams := internal.StrategyParams{VolumeMultiplier: 1.2}
	bestProfit := -1.0

	generator := s.GenerateSignals

	for mult := 1.0; mult <= 3.0; mult += 0.1 {
		params := internal.StrategyParams{VolumeMultiplier: mult}
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
	internal.RegisterStrategy("volume_breakout", &VolumeBreakoutStrategy{})
}
