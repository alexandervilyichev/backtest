// candle.go
package internal

import (
	"encoding/json"
	"log"
	"strconv"
	"time"
)

type Price float64

// UnmarshalJSON реализует пользовательский разбор JSON для Price.
// Преобразует объект {"units": "", "nano": 0} в float64 на этапе загрузки.
func (p *Price) UnmarshalJSON(data []byte) error {
	var temp struct {
		Units string `json:"units"`
		Nano  int32  `json:"nano"`
	}
	err := json.Unmarshal(data, &temp)
	if err != nil {
		return err
	}
	units, err := strconv.ParseInt(temp.Units, 10, 64)
	if err != nil {
		return err
	}
	*p = Price(float64(units) + float64(temp.Nano)/1_000_000_000.0)
	return nil
}

// ToFloat64 возвращает значение Price как float64.
// Теперь это простое приведение типов, поскольку преобразование происходит на этапе загрузки JSON.
func (p Price) ToFloat64() float64 {
	return float64(p)
}

func (c Candle) VolumeFloat64() float64 {
	v, err := strconv.ParseInt(c.Volume, 10, 64)
	if err != nil {
		log.Fatal("Invalid volume value:", c.Volume, err)
	}
	return float64(v)
}

type Candle struct {
	Open         Price     `json:"open"`
	High         Price     `json:"high"`
	Low          Price     `json:"low"`
	Close        Price     `json:"close"`
	Volume       string    `json:"volume"`
	Time         string    `json:"time"`
	IsComplete   bool      `json:"isComplete"`
	CandleSource string    `json:"candleSource"`
	ParsedTime   time.Time `json:"-"` // precomputed time for ToTime()
}

// GetCandlesResponse — ответ от API
type GetCandlesResponse struct {
	Candles []Candle `json:"candles"`
}

// RequestBody — тело запроса к /GetCandles
type RequestBody struct {
	From             string `json:"from"`
	To               string `json:"to"`
	Interval         string `json:"interval"`
	InstrumentId     string `json:"instrumentId"`
	CandleSourceType string `json:"candleSourceType"`
	Limit            int    `json:"limit"`
}

func (c Candle) ToTime() time.Time {
	// Возвращаем precomputed time.Time для оптимизации
	// Преобразование выполняется один раз на этапе загрузки JSON
	return c.ParsedTime
}

type SignalType int

const (
	HOLD SignalType = iota
	BUY
	SELL
)

func (s SignalType) String() string {
	return [...]string{"HOLD", "BUY", "SELL"}[s]
}
