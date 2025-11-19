// strategies/support_line.go

// Support Line Strategy
//
// Описание стратегии:
// Стратегия основана на концепции уровней поддержки - ценовых уровнях, где ожидается
// разворот цены вверх из-за превышения спроса над предложением. Стратегия ищет моменты,
// когда цена приближается к уровню поддержки снизу и открывает длинные позиции.
//
// Как работает:
// - Рассчитывается скользящий минимум (support level) за заданный период lookback
// - Покупка: когда цена закрытия опускается ниже уровня поддержки или очень близко к нему
// - Продажа: когда цена пробивает уровень поддержки вниз (подтверждение слома поддержки)
// - Фиксация прибыли: при достижении предыдущего максимума или сильном росте
//
// Параметры:
// - LookbackPeriod: период для расчета скользящего минимума (обычно 10-30)
// - BuyThreshold: порог расстояния до поддержки для покупки (обычно 0.001-0.01 = 0.1-1%)
// - SellThreshold: порог пробоя поддержки для продажи (обычно 0.005-0.02 = 0.5-2%)
//
// Сигналы стратегии:
// - BUY: цена приближается к поддержке снизу (closePrice >= support * (1 - buyThreshold))
// - SELL: цена пробивает поддержку вниз (closePrice < support * (1 - sellThreshold))
// - HOLD: позиция удерживается до сигнала продажи
//
// Сильные стороны:
// - Логичная идея: покупка у поддержки с ожиданием отскока
// - Хорошо работает в восходящих трендах с коррекциями
// - Учитывает рыночную психологию (поддержка как уровень спроса)
// - Может быть эффективна в комбинации с объемом или momentum индикаторами
//
// Слабые стороны:
// - Поддержка может не сработать в сильных нисходящих трендах
// - Зависит от правильного определения периода lookback
// - Может давать ложные сигналы при пробое поддержки
// - Требует хорошего risk management из-за потенциальных стоп-лоссов
//
// Лучшие условия для применения:
// - Восходящие тренды с коррекциями
// - Средне- и долгосрочная торговля
// - Волатильные активы с четкими уровнями поддержки/сопротивления
// - В сочетании с объемом или momentum индикаторами

package lines

import (
	"bt/internal"
	"errors"
	"fmt"

	"github.com/samber/lo"
)

type SupportLineConfig struct {
	LookbackPeriod int     `json:"lookback_period"`
	BuyThreshold   float64 `json:"buy_threshold"`
	SellThreshold  float64 `json:"sell_threshold"`
}

func (c *SupportLineConfig) Validate() error {
	if c.LookbackPeriod <= 0 {
		return errors.New("lookback period must be positive")
	}
	if c.BuyThreshold <= 0 || c.BuyThreshold >= 1.0 {
		return errors.New("buy threshold must be between 0 and 1.0")
	}
	if c.SellThreshold <= 0 || c.SellThreshold >= 1.0 {
		return errors.New("sell threshold must be between 0 and 1.0")
	}

	if c.SellThreshold <= c.BuyThreshold {
		return errors.New("sell threshold must be greater than buy threshold")
	}
	return nil
}

func (c *SupportLineConfig) String() string {
	return fmt.Sprintf("SupportLine(lookback=%d, buy_thresh=%.4f, sell_thresh=%.4f)",
		c.LookbackPeriod, c.BuyThreshold, c.SellThreshold)
}

type SupportLineSignalGenerator struct{}

func NewSupportLineSignalGenerator() *SupportLineSignalGenerator {
	return &SupportLineSignalGenerator{}
}

func (s *SupportLineSignalGenerator) GenerateSignals(candles []internal.Candle, config internal.StrategyConfigV2) []internal.SignalType {
	supportConfig, ok := config.(*SupportLineConfig)
	if !ok {
		return make([]internal.SignalType, len(candles))
	}

	if err := supportConfig.Validate(); err != nil {
		return make([]internal.SignalType, len(candles))
	}

	supportLevels := internal.CalculateRollingMin(candles, supportConfig.LookbackPeriod)
	if supportLevels == nil {
		return make([]internal.SignalType, len(candles))
	}

	signals := make([]internal.SignalType, len(candles))
	inPosition := false
	var entryPrice float64

	for i := supportConfig.LookbackPeriod; i < len(candles); i++ {
		support := supportLevels[i]
		closePrice := candles[i].Close.ToFloat64()

		// BUY сигнал: цена приближается к поддержке снизу
		// closePrice >= support * (1 - buyThreshold) означает цена выше поддержки минус порог
		if !inPosition && closePrice >= support*(1-supportConfig.BuyThreshold) {
			signals[i] = internal.BUY
			inPosition = true
			entryPrice = closePrice
			continue
		}

		if inPosition {
			// SELL сигнал: цена пробивает поддержку вниз
			if closePrice < support*(1-supportConfig.SellThreshold) {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}

			// Дополнительная фиксация прибыли при сильном росте (более 5% от входа)
			if closePrice >= entryPrice*1.05 {
				signals[i] = internal.SELL
				inPosition = false
				continue
			}
		}

		signals[i] = internal.HOLD
	}

	return signals
}

type SupportLineConfigGenerator struct{}

func NewSupportLineConfigGenerator() *SupportLineConfigGenerator {
	return &SupportLineConfigGenerator{}
}

func (s *SupportLineConfigGenerator) Generate() []internal.StrategyConfigV2 {

	configs := lo.CrossJoinBy3(
		lo.RangeWithSteps[int](40, 50, 1),
		lo.RangeWithSteps[float64](0.001, 0.02, 0.002),
		lo.RangeWithSteps[float64](0.001, 0.03, 0.0002),
		func(lookback int, buyThresh float64, sellThresh float64) internal.StrategyConfigV2 {
			return &SupportLineConfig{
				LookbackPeriod: lookback,
				BuyThreshold:   buyThresh,
				SellThreshold:  sellThresh,
			}
		})

	return configs
}

func NewSupportLineStrategyV2(slippage float64) internal.TradingStrategy {
	// 1. Создаем провайдер проскальзывания
	slippageProvider := internal.NewSlippageProvider(slippage)

	// 2. Создаем генератор сигналов
	signalGenerator := NewSupportLineSignalGenerator()

	// 3. Создаем менеджер конфигурации
	configManager := internal.NewConfigManager(
		&SupportLineConfig{},
		func() internal.StrategyConfigV2 {
			return &SupportLineConfig{}
		},
	)

	// 4. Создаем генератор конфигураций для оптимизации
	configGenerator := NewSupportLineConfigGenerator()

	// 5. Создаем оптимизатор (переиспользуем универсальный GridSearchOptimizer!)
	optimizer := internal.NewGridSearchOptimizer(
		slippageProvider,
		configGenerator.Generate,
	)

	// 6. Собираем всё вместе через композицию
	return internal.NewStrategyBase(
		"support_line_v2",
		signalGenerator,
		configManager,
		optimizer,
		slippageProvider,
	)
}

func init() {
	strategy := NewSupportLineStrategyV2(0.01) // default slippage 0.01
	internal.RegisterStrategyV2(strategy)
}
