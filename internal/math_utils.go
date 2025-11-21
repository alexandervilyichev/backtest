package internal

import "math"

// Abs возвращает абсолютное значение числа
func Abs(x float64) float64 {
	return math.Abs(x)
}

// Min возвращает минимум из двух чисел
func Min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Max возвращает максимум из двух чисел
func Max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
