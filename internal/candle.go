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
	if c.VolumeFloat == 0 && c.Volume != "0" && c.Volume != "" {
		// Fallback parsing if VolumeFloat wasn't set during JSON unmarshaling
		if v, err := strconv.ParseInt(c.Volume, 10, 64); err == nil {
			return float64(v)
		}
	}
	return c.VolumeFloat // return precomputed value
}

// UnmarshalJSON реализует пользовательский разбор JSON для Candle.
// Вызывается автоматически при json.Unmarshal для каждого элемента массива []Candle.
func (c *Candle) UnmarshalJSON(data []byte) error {
	type Alias Candle // создаем алиас для избежания бесконечной рекурсии
	aux := &struct {
		Time   string `json:"time"`
		Volume string `json:"volume"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}

	// Первый раз разбираем всё стандартными средствами
	err := json.Unmarshal(data, aux)
	if err != nil {
		return err
	}

	// Парсим время один раз и сохраняем в precomputed поле
	c.ParsedTime = time.Time{} // По умолчанию - нулевое время

	if aux.Time != "" {
		// Сначала пробуем RFC3339 с timezone
		parsedTime, err := time.Parse(time.RFC3339, aux.Time)
		if err != nil {
			// Если не получилось, пробуем RFC3339Nano
			parsedTime, err = time.Parse(time.RFC3339Nano, aux.Time)
			if err != nil {
				// Если не получилось, пробуем без timezone
				parsedTime, err = time.Parse("2006-01-02T15:04:05", aux.Time)
				if err != nil {
					log.Printf("❌ Все форматы времени провалились для: '%s', используем zero time", aux.Time)
				} else {
					c.ParsedTime = parsedTime
				}
			} else {
				c.ParsedTime = parsedTime
			}
		} else {
			c.ParsedTime = parsedTime
		}
	}

	// Преобразуем Volume из string в float64 один раз при загрузке
	vol, err := strconv.ParseInt(aux.Volume, 10, 64)
	if err != nil {
		log.Printf("Failed to parse volume: %s, error: %v", aux.Volume, err)
		c.VolumeFloat = 0.0 // присваиваем 0 в случае ошибки
	} else {
		c.VolumeFloat = float64(vol)
	}

	return nil
}

type Candle struct {
	Open         Price     `json:"open"`
	High         Price     `json:"high"`
	Low          Price     `json:"low"`
	Close        Price     `json:"close"`
	Volume       string    `json:"volume"`
	VolumeFloat  float64   `json:"-"` // precomputed float64 volume for performance
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
