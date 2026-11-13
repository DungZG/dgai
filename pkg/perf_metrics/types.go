package perfmetrics

import (
	"math"
	"sync/atomic"
)

type Store interface {
	Record(sample Sample)
	Query(params QueryParams) (QueryResult, error)
}

type Sample struct {
	Model        string
	Group        string
	LatencyMs    int64
	TtftMs       int64
	HasTtft      bool
	Success      bool
	OutputTokens int64
	GenerationMs int64
}

type QueryParams struct {
	Model string
	Group string
	Hours int
	// AllowedGroups restricts both the per-group results and the model summary;
	// nil allows every group.
	AllowedGroups []string
}

// Summary is the request-weighted aggregate over every bucket in the window.
type Summary struct {
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	P50LatencyMs int64   `json:"p50_latency_ms"`
	P95LatencyMs int64   `json:"p95_latency_ms"`
	P50TtftMs    int64   `json:"p50_ttft_ms"`
	P95TtftMs    int64   `json:"p95_ttft_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
}

type BucketPoint struct {
	Ts           int64   `json:"ts"`
	AvgTtftMs    int64   `json:"avg_ttft_ms"`
	AvgLatencyMs int64   `json:"avg_latency_ms"`
	P50LatencyMs int64   `json:"p50_latency_ms"`
	P95LatencyMs int64   `json:"p95_latency_ms"`
	P50TtftMs    int64   `json:"p50_ttft_ms"`
	P95TtftMs    int64   `json:"p95_ttft_ms"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTps       float64 `json:"avg_tps"`
}

type GroupResult struct {
	Group        string        `json:"group"`
	AvgTtftMs    int64         `json:"avg_ttft_ms"`
	AvgLatencyMs int64         `json:"avg_latency_ms"`
	P50LatencyMs int64         `json:"p50_latency_ms"`
	P95LatencyMs int64         `json:"p95_latency_ms"`
	P50TtftMs    int64         `json:"p50_ttft_ms"`
	P95TtftMs    int64         `json:"p95_ttft_ms"`
	SuccessRate  float64       `json:"success_rate"`
	AvgTps       float64       `json:"avg_tps"`
	Series       []BucketPoint `json:"series"`
}

type QueryResult struct {
	ModelName    string        `json:"model_name"`
	SeriesSchema string        `json:"series_schema"`
	Summary      *Summary      `json:"summary"`
	Series       []BucketPoint `json:"series"`
	WindowStart  int64         `json:"window_start"`
	WindowEnd    int64         `json:"window_end"`
	Groups       []GroupResult `json:"groups"`
}

type SuccessRatePoint struct {
	Ts          int64   `json:"ts"`
	SuccessRate float64 `json:"success_rate"`
}

type ModelSummary struct {
	ModelName           string             `json:"model_name"`
	AvgLatencyMs        int64              `json:"avg_latency_ms"`
	P50LatencyMs        int64              `json:"p50_latency_ms"`
	P95LatencyMs        int64              `json:"p95_latency_ms"`
	P50TtftMs           int64              `json:"p50_ttft_ms"`
	P95TtftMs           int64              `json:"p95_ttft_ms"`
	SuccessRate         float64            `json:"success_rate"`
	AvgTps              float64            `json:"avg_tps"`
	RecentSuccessSeries []SuccessRatePoint `json:"recent_success_series,omitempty"`
	RequestCount        int64              `json:"-"`
}

type SummaryAllResult struct {
	Summary     *Summary       `json:"summary"`
	WindowStart int64          `json:"window_start"`
	WindowEnd   int64          `json:"window_end"`
	Models      []ModelSummary `json:"models"`
	// RecentSuccessSeries contains up to 24 nonempty hours from the last 7 days,
	// including requested models with no traffic in the summary window.
	RecentSuccessSeries map[string][]SuccessRatePoint `json:"recent_success_series,omitempty"`
}

type bucketKey struct {
	model    string
	group    string
	bucketTs int64
}

type counters struct {
	requestCount   int64
	successCount   int64
	totalLatencyMs int64
	ttftSumMs      int64
	ttftCount      int64
	outputTokens   int64
	generationMs   int64
	latencyHist    [latencyHistBuckets]int64
	ttftHist       [latencyHistBuckets]int64
}

type atomicBucket struct {
	requestCount   atomic.Int64
	successCount   atomic.Int64
	totalLatencyMs atomic.Int64
	ttftSumMs      atomic.Int64
	ttftCount      atomic.Int64
	outputTokens   atomic.Int64
	generationMs   atomic.Int64
	latencyHist    [latencyHistBuckets]atomic.Int64
	ttftHist       [latencyHistBuckets]atomic.Int64
}

func (b *atomicBucket) add(sample Sample) {
	b.requestCount.Add(1)
	if sample.Success {
		b.successCount.Add(1)
	}
	if sample.LatencyMs > 0 {
		b.totalLatencyMs.Add(sample.LatencyMs)
		b.latencyHist[latencyHistIndex(sample.LatencyMs)].Add(1)
	}
	if sample.HasTtft && sample.TtftMs >= 0 {
		b.ttftSumMs.Add(sample.TtftMs)
		b.ttftCount.Add(1)
		b.ttftHist[latencyHistIndex(sample.TtftMs)].Add(1)
	}
	if sample.OutputTokens > 0 && sample.GenerationMs > 0 {
		b.outputTokens.Add(sample.OutputTokens)
		b.generationMs.Add(sample.GenerationMs)
	}
}

func (b *atomicBucket) snapshot() counters {
	return counters{
		requestCount:   b.requestCount.Load(),
		successCount:   b.successCount.Load(),
		totalLatencyMs: b.totalLatencyMs.Load(),
		ttftSumMs:      b.ttftSumMs.Load(),
		ttftCount:      b.ttftCount.Load(),
		outputTokens:   b.outputTokens.Load(),
		generationMs:   b.generationMs.Load(),
		latencyHist:    b.histSnapshot(&b.latencyHist, false),
		ttftHist:       b.histSnapshot(&b.ttftHist, false),
	}
}

func (b *atomicBucket) drain() counters {
	return counters{
		requestCount:   b.requestCount.Swap(0),
		successCount:   b.successCount.Swap(0),
		totalLatencyMs: b.totalLatencyMs.Swap(0),
		ttftSumMs:      b.ttftSumMs.Swap(0),
		ttftCount:      b.ttftCount.Swap(0),
		outputTokens:   b.outputTokens.Swap(0),
		generationMs:   b.generationMs.Swap(0),
		latencyHist:    b.histSnapshot(&b.latencyHist, true),
		ttftHist:       b.histSnapshot(&b.ttftHist, true),
	}
}

func (b *atomicBucket) histSnapshot(src *[latencyHistBuckets]atomic.Int64, drain bool) [latencyHistBuckets]int64 {
	var hist [latencyHistBuckets]int64
	for i := range src {
		if drain {
			hist[i] = src[i].Swap(0)
		} else {
			hist[i] = src[i].Load()
		}
	}
	return hist
}

func (b *atomicBucket) addCounters(c counters) {
	if c.requestCount != 0 {
		b.requestCount.Add(c.requestCount)
	}
	if c.successCount != 0 {
		b.successCount.Add(c.successCount)
	}
	if c.totalLatencyMs != 0 {
		b.totalLatencyMs.Add(c.totalLatencyMs)
	}
	if c.ttftSumMs != 0 {
		b.ttftSumMs.Add(c.ttftSumMs)
	}
	if c.ttftCount != 0 {
		b.ttftCount.Add(c.ttftCount)
	}
	if c.outputTokens != 0 {
		b.outputTokens.Add(c.outputTokens)
	}
	if c.generationMs != 0 {
		b.generationMs.Add(c.generationMs)
	}
	for i, n := range c.latencyHist {
		if n != 0 {
			b.latencyHist[i].Add(n)
		}
	}
	for i, n := range c.ttftHist {
		if n != 0 {
			b.ttftHist[i].Add(n)
		}
	}
}

// latencyHistBuckets buckets latency into fixed upper bounds so percentiles stay
// mergeable across the in-memory and Redis stores. Percentiles report the upper
// bound of the bucket that crosses the target, so P95 is never understated.
const latencyHistBuckets = 24

var latencyHistBounds = [latencyHistBuckets]int64{
	50, 100, 150, 200, 300, 400, 500, 750,
	1000, 1500, 2000, 3000, 4000, 5000, 7500, 10000,
	15000, 20000, 30000, 45000, 60000, 90000, 120000, 300000,
}

func latencyHistIndex(ms int64) int {
	for i, bound := range latencyHistBounds {
		if ms <= bound {
			return i
		}
	}
	return latencyHistBuckets - 1
}

// latencyPercentile returns the bucket upper bound at which the cumulative
// request count reaches q (0..1). Zero when the histogram is empty.
func latencyPercentile(hist *[latencyHistBuckets]int64, q float64) int64 {
	var total int64
	for _, n := range hist {
		total += n
	}
	if total <= 0 {
		return 0
	}
	target := int64(math.Ceil(q * float64(total)))
	if target < 1 {
		target = 1
	}
	var seen int64
	for i, n := range hist {
		seen += n
		if seen >= target {
			return latencyHistBounds[i]
		}
	}
	return latencyHistBounds[latencyHistBuckets-1]
}
