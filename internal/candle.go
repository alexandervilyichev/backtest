// candle.go
package internal

import (
	"log"
	"strconv"
	"time"
)

type Price struct {
	Units string `json:"units"`
	Nano  int32  `json:"nano"`
}

func (p Price) ToFloat64() float64 {
	units, err := strconv.ParseInt(p.Units, 10, 64)
	if err != nil {
		log.Fatal("Invalid units value:", p.Units, err)
	}
	return float64(units) + float64(p.Nano)/1_000_000_000.0
}

func (c Candle) VolumeFloat64() float64 {
	v, err := strconv.ParseInt(c.Volume, 10, 64)
	if err != nil {
		log.Fatal("Invalid volume value:", c.Volume, err)
	}
	return float64(v)
}

type Candle struct {
	Open         Price  `json:"open"`
	High         Price  `json:"high"`
	Low          Price  `json:"low"`
	Close        Price  `json:"close"`
	Volume       string `json:"volume"`
	Time         string `json:"time"`
	IsComplete   bool   `json:"isComplete"`
	CandleSource string `json:"candleSource"`
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
	t, err := time.Parse(time.RFC3339, c.Time)
	if err != nil {
		log.Fatal("Invalid time format:", c.Time)
	}
	return t
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
