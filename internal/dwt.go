package internal

import (
	"errors"
	"fmt"
	"math"
)

// --- Константы для вейвлета Daubechies 4 ---
var (
	db4H0 = (1 + math.Sqrt(3)) / (4 * math.Sqrt(2))
	db4H1 = (3 + math.Sqrt(3)) / (4 * math.Sqrt(2))
	db4H2 = (3 - math.Sqrt(3)) / (4 * math.Sqrt(2))
	db4H3 = (1 - math.Sqrt(3)) / (4 * math.Sqrt(2))
)

// db4Coefficients возвращает коэффициенты фильтров для вейвлета Daubechies 4.
// h - коэффициенты масштабирующего (низкочастотного) фильтра.
// g - коэффициенты вейвлетного (высокочастотного) фильтра.
func db4Coefficients() (h, g []float64) {
	h = []float64{db4H0, db4H1, db4H2, db4H3}
	g = []float64{-db4H3, -db4H2, db4H1, -db4H0}
	return
}

// periodicExtend расширяет сигнал периодически на ext точек с каждой стороны.
func periodicExtend(signal []float64, ext int) []float64 {
	if len(signal) == 0 {
		return []float64{}
	}
	n := len(signal)
	extended := make([]float64, n+2*ext)
	for i := 0; i < len(extended); i++ {
		// Используем модульную арифметику для циклического доступа
		idx := (i - ext) % n
		if idx < 0 {
			idx += n
		}
		extended[i] = signal[idx]
	}
	return extended
}

// convolve1D выполняет 1D свертку сигнала с фильтром.
// Использует периодическое расширение сигнала.
func convolve1D(signal, filter []float64) []float64 {
	if len(signal) == 0 || len(filter) == 0 {
		return []float64{}
	}
	ext := len(filter) - 1
	extended := periodicExtend(signal, ext)
	n := len(extended)
	m := len(filter)
	// Результирующий массив будет длиной n - m + 1 (режим "valid")
	result := make([]float64, n-m+1)
	for i := 0; i < len(result); i++ {
		for j := 0; j < m; j++ {
			result[i] += extended[i+j] * filter[j]
		}
	}
	return result
}

// downsample выбирает каждый 2-й элемент, начиная с start.
func downsample(signal []float64, start int) []float64 {
	if start < 0 {
		return []float64{}
	}
	n := len(signal)
	// (n - start + 1) потому что последний элемент может быть включен
	// (n - start + 1) / 2 - это целочисленное деление, оно корректно
	result := make([]float64, (n-start+1)/2)
	for i := start; i < n; i += 2 {
		result[(i-start)/2] = signal[i]
	}
	return result
}

// upsample вставляет нули между элементами сигнала.
func upsample(signal []float64) []float64 {
	n := len(signal)
	result := make([]float64, 2*n)
	for i := 0; i < n; i++ {
		result[2*i] = signal[i]
		// result[2*i+1] уже равно 0 по умолчанию
	}
	return result
}

// DWT выполняет одностороннее (один уровень) дискретное вейвлет-преобразование (DWT).
// Использует вейвлет Daubechies 4.
// signal: входной сигнал. Длина должна быть четной.
// Возвращает:
// - approx: коэффициенты аппроксимации (низкочастотная часть).
// - detail: коэффициенты детализации (высокочастотная часть).
// Возвращает ошибку, если длина сигнала нечетная.
func DWT(signal []float64) (approx, detail []float64, err error) {
	if len(signal) == 0 {
		return []float64{}, []float64{}, nil
	}
	if len(signal)%2 != 0 {
		return nil, nil, errors.New("DWT: signal length must be even")
	}

	h, g := db4Coefficients()

	// 1. Свертка сигнала с фильтрами h и g
	convH := convolve1D(signal, h)
	convG := convolve1D(signal, g)

	// 2. Определение среза, соответствующего "same" размеру
	ext := len(h) - 1
	sameStart := ext
	sameEnd := sameStart + len(signal)

	// Проверка на случай, если индексы выходят за границы (хотя в норме не должны)
	if sameEnd > len(convH) || sameEnd > len(convG) {
		return nil, nil, fmt.Errorf("DWT: internal convolution length mismatch")
	}

	// 3. Прореживание (downsampling)
	approx = downsample(convH[sameStart:sameEnd], 0)
	detail = downsample(convG[sameStart:sameEnd], 0)

	return approx, detail, nil
}

// IDWT выполняет одностороннее (один уровень) обратное дискретное вейвлет-преобразование (IDWT).
// Использует вейвлет Daubechies 4.
// approx: коэффициенты аппроксимации.
// detail: коэффициенты детализации.
// Возвращает восстановленный сигнал.
// Возвращает ошибку, если длины approx и detail не совпадают.
func IDWT(approx, detail []float64) ([]float64, error) {
	if len(approx) != len(detail) {
		return nil, errors.New("IDWT: approximation and detail coefficients must have the same length")
	}
	if len(approx) == 0 {
		return []float64{}, nil
	}

	h, g := db4Coefficients()

	// 1. Разворот фильтров для реконструкции
	hRev := []float64{h[3], h[2], h[1], h[0]}
	gRev := []float64{g[3], g[2], g[1], g[0]}

	// 2. Интерполяция (upsampling)
	upApprox := upsample(approx)
	upDetail := upsample(detail)

	// 3. Свертка с реверсивными фильтрами
	convH := convolve1D(upApprox, hRev)
	convG := convolve1D(upDetail, gRev)

	// 4. Суммирование
	if len(convH) != len(convG) {
		// Это внутренняя ошибка, не должна возникнуть при корректной реализации
		return nil, fmt.Errorf("IDWT: internal convolution output length mismatch")
	}

	n := len(convH)
	sumSignal := make([]float64, n)
	for i := 0; i < n; i++ {
		sumSignal[i] = convH[i] + convG[i]
	}

	// 5. Выбор центральной части (для компенсации сдвига)
	// В оригинальной реализации предполагался сдвиг на 3.
	// Для db4 (длина фильтра 4), сдвиг в реконструкции часто составляет (length_filter - 1) / 2 = 1.5, округляем до 1 или 2.
	// В оригинальном коде использовалось смещение 3. Это может быть связано с деталями реализации свертки.
	// Проверим, какая длина должна быть у результата: 2 * len(approx).
	// convH и convG имеют длину 2 * len(approx) + (len(filter) - 1) = 2*L + 3.
	// sumSignal также длины 2*L + 3.
	// Нам нужно 2*L. Значит, нужно отрезать по 3/2 с каждой стороны, или 3 с одной и 0 с другой.
	// Оригинальный код: start = 3, length = 2*L.
	// Проверим, что это дает корректную реконструкцию.
	start := 3
	expectedLength := 2 * len(approx)
	if start+expectedLength > len(sumSignal) {
		return nil, fmt.Errorf("IDWT: insufficient data for reconstruction slice")
	}

	reconstructed := sumSignal[start : start+expectedLength]

	return reconstructed, nil
}
