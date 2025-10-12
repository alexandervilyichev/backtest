// // linear_regression.go
package internal

// import (
// 	"fmt"
// )

// // LinearRegression — простая линейная модель с градиентным спуском
// type LinearRegression struct {
// 	Weights      []float64
// 	LearningRate float64
// 	Iterations   int
// }

// func NewLinearRegression(nFeatures int, lr float64, iters int) *LinearRegression {
// 	return &LinearRegression{
// 		Weights:      make([]float64, nFeatures+1), // +1 for bias
// 		LearningRate: lr,
// 		Iterations:   iters,
// 	}
// }

// // Train обучает модель на X (признаки) и y (целевые значения)
// func (lr *LinearRegression) Train(X [][]float64, y []float64) {
// 	nSamples := len(X)
// 	nFeatures := len(X[0])

// 	for iter := 0; iter < lr.Iterations; iter++ {
// 		predictions := lr.PredictBatch(X)
// 		loss := lr.computeLoss(predictions, y)

// 		// Градиенты
// 		gradients := make([]float64, nFeatures+1)
// 		for j := 0; j <= nFeatures; j++ {
// 			sum := 0.0
// 			for i := 0; i < nSamples; i++ {
// 				pred := predictions[i]
// 				actual := y[i]
// 				xji := 1.0
// 				if j > 0 {
// 					xji = X[i][j-1]
// 				}
// 				sum += (pred - actual) * xji
// 			}
// 			gradients[j] = sum / float64(nSamples)
// 		}

// 		// Обновляем веса
// 		for j := 0; j <= nFeatures; j++ {
// 			lr.Weights[j] -= lr.LearningRate * gradients[j]
// 		}

// 		if iter%100 == 0 {
// 			fmt.Printf("Итерация %d: Loss=%.6f\n", iter, loss)
// 		}
// 	}
// }

// // PredictSingle предсказывает для одного примера
// func (lr *LinearRegression) PredictSingle(features []float64) float64 {
// 	pred := lr.Weights[0] // bias
// 	for i, f := range features {
// 		pred += lr.Weights[i+1] * f
// 	}
// 	return pred
// }

// // predictBatch предсказывает для всего батча
// func (lr *LinearRegression) PredictBatch(X [][]float64) []float64 {
// 	predictions := make([]float64, len(X))
// 	for i, x := range X {
// 		predictions[i] = lr.PredictSingle(x)
// 	}
// 	return predictions
// }

// // computeLoss MSE
// func (lr *LinearRegression) computeLoss(predictions, y []float64) float64 {
// 	sum := 0.0
// 	for i := range predictions {
// 		diff := predictions[i] - y[i]
// 		sum += diff * diff
// 	}
// 	return sum / float64(len(predictions))
// }

// // PredictClass — адаптивный порог на основе медианы предсказаний
// func (lr *LinearRegression) PredictClass(features []float64, trainPreds []float64) SignalType {
// 	pred := lr.PredictSingle(features)
// 	medianPred := Median(trainPreds)

// 	// Минимальная сила сигнала — чтобы не торговать на шуме
// 	minStrength := 0.01 // ← УМЕНЬШИЛИ!

// 	if pred > medianPred+minStrength {
// 		return BUY
// 	} else if pred < medianPred-minStrength {
// 		return SELL
// 	}
// 	return HOLD
// }
