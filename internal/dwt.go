package internal

import (
	"math"
)

// Daubechies4Coefficients returns the scaling (low-pass) and wavelet (high-pass) coefficients for Daubechies 4 wavelet
func Daubechies4Coefficients() (h, g []float64) {
	sqrt3 := math.Sqrt(3)
	h0 := (1 + sqrt3) / (4 * math.Sqrt(2))
	h1 := (3 + sqrt3) / (4 * math.Sqrt(2))
	h2 := (3 - sqrt3) / (4 * math.Sqrt(2))
	h3 := (1 - sqrt3) / (4 * math.Sqrt(2))

	h = []float64{h0, h1, h2, h3}
	g = []float64{-h3, -h2, h1, -h0}
	return
}

// periodicExtend extends the signal periodically by ext points on each side
func periodicExtend(signal []float64, ext int) []float64 {
	n := len(signal)
	extended := make([]float64, n+2*ext)
	for i := 0; i < len(extended); i++ {
		extended[i] = signal[(i-ext+n)%n]
	}
	return extended
}

// convolve performs 1D full convolution with periodic extension
func convolve(signal, filter []float64) []float64 {
	ext := len(filter) - 1
	extended := periodicExtend(signal, ext)
	n := len(extended)
	m := len(filter)
	result := make([]float64, n-m+1)
	for i := 0; i < len(result); i++ {
		for j := 0; j < m; j++ {
			result[i] += extended[i+j] * filter[j]
		}
	}
	return result
}

// downsample takes every 2nd element starting from start
func downsample(signal []float64, start int) []float64 {
	n := len(signal)
	result := make([]float64, (n-start+1)/2)
	for i := start; i < n; i += 2 {
		result[(i-start)/2] = signal[i]
	}
	return result
}

// upsample inserts zeros
func upsample(signal []float64) []float64 {
	n := len(signal)
	result := make([]float64, 2*n)
	for i := 0; i < n; i++ {
		result[2*i] = signal[i]
	}
	return result
}

// DWT performs single level Discrete Wavelet Transform on signal using db4
// Returns approximation and detail coefficients
func DWT(signal []float64) (approx, detail []float64) {
	h, g := Daubechies4Coefficients()

	// Convolve with h and g
	convH := convolve(signal, h)
	convG := convolve(signal, g)

	n := len(signal)
	if n%2 != 0 {
		panic("Signal length must be even for single level DWT")
	}

	ext := len(h) - 1
	sameStart := ext
	sameEnd := sameStart + n

	// Downsample the 'same' part
	approx = downsample(convH[sameStart:sameEnd], 0)
	detail = downsample(convG[sameStart:sameEnd], 0)

	return
}

// IDWT performs single level Inverse Discrete Wavelet Transform
func IDWT(approx, detail []float64) []float64 {
	h, g := Daubechies4Coefficients()

	// Reverse the filters for reconstruction
	hRev := make([]float64, len(h))
	gRev := make([]float64, len(g))
	for i := 0; i < len(h); i++ {
		hRev[i] = h[len(h)-1-i]
		gRev[i] = g[len(g)-1-i]
	}

	// Upsample
	upApprox := upsample(approx)
	upDetail := upsample(detail)

	// Convolve
	convH := convolve(upApprox, hRev)
	convG := convolve(upDetail, gRev)

	// Sum
	n := len(convH)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = convH[i] + convG[i]
	}

	// Take the middle part, from 3 to 3 + 2*len(approx) -1 or something.

	// For db4, the reconstruction starts from index 3, length 2*len(approx)

	start := 3
	length := 2 * len(approx)
	return result[start : start+length]
}
