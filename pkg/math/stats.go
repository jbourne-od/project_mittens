package math

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrInsufficientSamples is returned when fewer than 2 samples are provided for statistical testing.
	ErrInsufficientSamples = errors.New("stats: at least 2 samples are required")
	// ErrMismatchedSampleSizes is returned when paired sample slices have unequal lengths.
	ErrMismatchedSampleSizes = errors.New("stats: paired sample slices must have equal length")
)

// PairedTTestResult encapsulates the statistical summary of a paired hypothesis test.
type PairedTTestResult struct {
	N                int     // Number of paired observations
	MeanBaseline     float64 // Sample mean of baseline (N=0)
	MeanCandidate    float64 // Sample mean of candidate (N=1)
	MeanDifference   float64 // \bar{d} = MeanCandidate - MeanBaseline
	StdDevDifference float64 // Sample standard deviation of differences s_d
	StdErrDifference float64 // Standard error of mean difference s_d / \sqrt{N}
	TStatistic       float64 // Student's t-statistic
	DegreesOfFreedom float64 // df = N - 1
	PValueOneTailed  float64 // Pr(T >= t | H_0) (testing Candidate > Baseline)
	PValueTwoTailed  float64 // Pr(|T| >= |t| | H_0)
	ConfidenceLow95  float64 // Lower bound of 95% confidence interval for \mu_d
	ConfidenceHigh95 float64 // Upper bound of 95% confidence interval for \mu_d
	PercentLift      float64 // (\bar{d} / |\bar{x}_{\text{base}}|) * 100
}

// SummaryString formats the test result as a human-readable statistical report.
func (r PairedTTestResult) SummaryString() string {
	return fmt.Sprintf(
		"Paired t-Test (N=%d, df=%.0f):\n"+
			"  Baseline Mean:   $%.2f\n"+
			"  Candidate Mean:  $%.2f\n"+
			"  Mean Difference: $%.2f (Lift: +%.2f%%)\n"+
			"  95%% CI:          [$%.2f, $%.2f]\n"+
			"  t-Statistic:     %.4f\n"+
			"  p-Value (1-tail): %e\n"+
			"  p-Value (2-tail): %e",
		r.N, r.DegreesOfFreedom,
		r.MeanBaseline, r.MeanCandidate,
		r.MeanDifference, r.PercentLift,
		r.ConfidenceLow95, r.ConfidenceHigh95,
		r.TStatistic,
		r.PValueOneTailed,
		r.PValueTwoTailed,
	)
}

// ComputePairedTTest performs a paired Student's t-test comparing candidate values against baseline values.
// Mathematical Formulation:
//  1. Differences: d_i = candidate_i - baseline_i for i = 1..N
//  2. Mean difference: \bar{d} = \frac{1}{N} \sum d_i
//  3. Variance of difference: s_d^2 = \frac{1}{N-1} \sum (d_i - \bar{d})^2
//  4. t-statistic: t = \frac{\bar{d}}{s_d / \sqrt{N}}
//  5. Degrees of freedom: df = N - 1
func ComputePairedTTest(baseline, candidate []float64) (PairedTTestResult, error) {
	if len(baseline) != len(candidate) {
		return PairedTTestResult{}, ErrMismatchedSampleSizes
	}
	n := len(baseline)
	if n < 2 {
		return PairedTTestResult{}, ErrInsufficientSamples
	}

	sumBase := 0.0
	sumCand := 0.0
	diffs := make([]float64, n)
	sumDiff := 0.0

	for i := 0; i < n; i++ {
		b := baseline[i]
		c := candidate[i]
		if math.IsNaN(b) || math.IsNaN(c) || math.IsInf(b, 0) || math.IsInf(c, 0) {
			return PairedTTestResult{}, fmt.Errorf("stats: non-finite value at index %d (base=%v, cand=%v)", i, b, c)
		}
		sumBase += b
		sumCand += c
		d := c - b
		diffs[i] = d
		sumDiff += d
	}

	meanBase := sumBase / float64(n)
	meanCand := sumCand / float64(n)
	meanDiff := sumDiff / float64(n)

	sumSqDiff := 0.0
	for _, d := range diffs {
		dev := d - meanDiff
		sumSqDiff += dev * dev
	}

	varianceDiff := sumSqDiff / float64(n-1)
	stdDevDiff := math.Sqrt(varianceDiff)
	stdErrDiff := stdDevDiff / math.Sqrt(float64(n))

	tStat := 0.0
	if stdErrDiff > 1e-15 {
		tStat = meanDiff / stdErrDiff
	}

	df := float64(n - 1)
	pTwoTailed := StudentTCDFTwoTailed(math.Abs(tStat), df)
	pOneTailed := pTwoTailed / 2.0
	if tStat < 0 {
		pOneTailed = 1.0 - pOneTailed
	}

	// 95% critical value approximation for Student's t
	tCrit := StudentTCriticalValue(0.05, df)
	marginOfError := tCrit * stdErrDiff

	percentLift := 0.0
	if math.Abs(meanBase) > 1e-9 {
		percentLift = (meanDiff / math.Abs(meanBase)) * 100.0
	}

	return PairedTTestResult{
		N:                n,
		MeanBaseline:     meanBase,
		MeanCandidate:    meanCand,
		MeanDifference:   meanDiff,
		StdDevDifference: stdDevDiff,
		StdErrDifference: stdErrDiff,
		TStatistic:       tStat,
		DegreesOfFreedom: df,
		PValueOneTailed:  pOneTailed,
		PValueTwoTailed:  pTwoTailed,
		ConfidenceLow95:  meanDiff - marginOfError,
		ConfidenceHigh95: meanDiff + marginOfError,
		PercentLift:      percentLift,
	}, nil
}

// StudentTCDFTwoTailed computes the two-tailed p-value Pr(|T| >= |t|) for Student's t-distribution with df degrees of freedom.
// Uses the relationship with the Regularized Incomplete Beta Function:
//
//	p = I_x(df/2, 1/2) where x = \frac{df}{df + t^2}
func StudentTCDFTwoTailed(absT, df float64) float64 {
	if df <= 0 {
		return 1.0
	}
	if absT <= 0 {
		return 1.0
	}
	x := df / (df + absT*absT)
	return IncompleteBeta(0.5*df, 0.5, x)
}

// normalQuantile approximates the standard normal inverse CDF z_p for p \in (0, 1)
// using the classical rational approximation (Abramowitz & Stegun 26.2.23).
func normalQuantile(p float64) float64 {
	if p <= 0.0 {
		return -1e10
	}
	if p >= 1.0 {
		return 1e10
	}
	if p == 0.5 {
		return 0.0
	}
	if p < 0.5 {
		return -normalQuantile(1.0 - p)
	}

	t := math.Sqrt(-2.0 * math.Log(1.0-p))
	c0 := 2.515517
	c1 := 0.802853
	c2 := 0.010328
	d1 := 1.432788
	d2 := 0.189269
	d3 := 0.001308

	numerator := c0 + t*(c1+t*c2)
	denominator := 1.0 + t*(d1+t*(d2+t*d3))
	return t - (numerator / denominator)
}

// StudentTCriticalValue approximates the two-tailed critical value t_{\alpha/2, df} for significance level alpha \in (0, 1).
// Utilizes Cornish-Fisher asymptotic expansion from the standard normal quantile.
func StudentTCriticalValue(alpha, df float64) float64 {
	if df <= 0 {
		return 1.96
	}
	if alpha <= 0 || alpha >= 1.0 {
		alpha = 0.05
	}
	p := 1.0 - alpha/2.0
	z := normalQuantile(p)
	z2 := z * z
	z3 := z2 * z
	z5 := z3 * z2

	// Cornish-Fisher expansion:
	// t \approx z + \frac{z^3 + z}{4 df} + \frac{5z^5 + 16z^3 + 3z}{96 df^2}
	term1 := (z3 + z) / (4.0 * df)
	term2 := (5.0*z5 + 16.0*z3 + 3.0*z) / (96.0 * df * df)
	return z + term1 + term2
}

// IncompleteBeta computes the regularized incomplete beta function I_x(a, b) = \frac{B(x; a, b)}{B(a, b)}.
// Evaluated using the continued fraction expansion (Abramowitz & Stegun 26.5.8).
func IncompleteBeta(a, b, x float64) float64 {
	if x <= 0.0 {
		return 0.0
	}
	if x >= 1.0 {
		return 1.0
	}
	if a <= 0 || b <= 0 {
		return 0.0
	}

	// Symmetry transformation to ensure faster convergence:
	// If x > (a + 1) / (a + b + 2), use I_x(a, b) = 1 - I_{1-x}(b, a)
	if x > (a+1.0)/(a+b+2.0) {
		return 1.0 - IncompleteBeta(b, a, 1.0-x)
	}

	// Calculate factor: \frac{x^a (1-x)^b}{a B(a, b)}
	lnBeta, _ := math.Lgamma(a)
	lnBetaB, _ := math.Lgamma(b)
	lnBetaAB, _ := math.Lgamma(a + b)
	logBeta := lnBeta + lnBetaB - lnBetaAB

	front := math.Exp(a*math.Log(x)+b*math.Log(1.0-x)-logBeta) / a

	// Continued fraction expansion (Modified Lentz's method)
	const maxIter = 200
	const eps = 1e-15
	const tiny = 1e-30

	c := 1.0
	d := 1.0 - (a+b)*x/(a+1.0)
	if math.Abs(d) < tiny {
		d = tiny
	}
	d = 1.0 / d
	h := d

	for m := 1; m <= maxIter; m++ {
		mF := float64(m)

		// Even step 2m:
		numEven := mF * (b - mF) * x / ((a + 2.0*mF - 1.0) * (a + 2.0*mF))
		d = 1.0 + numEven*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1.0 + numEven/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1.0 / d
		h *= d * c

		// Odd step 2m+1:
		numOdd := -(a + mF) * (a + b + mF) * x / ((a + 2.0*mF) * (a + 2.0*mF + 1.0))
		d = 1.0 + numOdd*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		c = 1.0 + numOdd/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		d = 1.0 / d
		del := d * c
		h *= del

		if math.Abs(del-1.0) < eps {
			break
		}
	}

	res := front * h
	if res < 0.0 {
		return 0.0
	}
	if res > 1.0 {
		return 1.0
	}
	return res
}
