package perfmetrics

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

func flushLoop() {
	for {
		interval := perf_metrics_setting.GetFlushIntervalMinutes()
		time.Sleep(time.Duration(interval) * time.Minute)
		setting := perf_metrics_setting.GetSetting()
		if !setting.Enabled {
			continue
		}
		flushCompletedBuckets()
		cleanupExpiredMetrics(setting.RetentionDays)
	}
}

func flushCompletedBuckets() {
	currentBucket := bucketStart(time.Now().Unix())
	hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs >= currentBucket {
			return true
		}

		bucket := value.(*atomicBucket)
		drained := bucket.drain()
		if drained.requestCount == 0 {
			deleteOldEmptyBucket(k, key)
			return true
		}

		err := model.UpsertPerfMetric(&model.PerfMetric{
			ModelName:      k.model,
			Group:          k.group,
			BucketTs:       k.bucketTs,
			RequestCount:   drained.requestCount,
			SuccessCount:   drained.successCount,
			TotalLatencyMs: drained.totalLatencyMs,
			TtftSumMs:      drained.ttftSumMs,
			TtftCount:      drained.ttftCount,
			OutputTokens:   drained.outputTokens,
			GenerationMs:   drained.generationMs,
			LatencyHist:    encodeLatencyHist(drained.latencyHist),
			TtftHist:       encodeLatencyHist(drained.ttftHist),
		})
		if err != nil {
			bucket.addCounters(drained)
			common.SysError(common.LogText("failed to flush perf metric bucket model=%s group=%s bucket=%d: %s", k.model, k.group, k.bucketTs, err.Error()))
			return true
		}

		deleteOldEmptyBucket(k, key)
		return true
	})
}

func deleteOldEmptyBucket(k bucketKey, rawKey any) {
	if k.bucketTs < bucketStart(time.Now().Add(-24*time.Hour).Unix()) {
		hotBuckets.Delete(rawKey)
	}
}

func cleanupExpiredMetrics(retentionDays int) {
	if retentionDays <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	if err := model.DeletePerfMetricsBefore(cutoff); err != nil {
		common.SysError(common.LogText("failed to cleanup expired perf metrics: %s", err.Error()))
	}
}

func redisCounters(values map[string]string) counters {
	result := counters{
		requestCount:   parseRedisInt(values["req"]),
		successCount:   parseRedisInt(values["ok"]),
		totalLatencyMs: parseRedisInt(values["lat"]),
		ttftSumMs:      parseRedisInt(values["ttft"]),
		ttftCount:      parseRedisInt(values["ttft_n"]),
		outputTokens:   parseRedisInt(values["out"]),
		generationMs:   parseRedisInt(values["gen_ms"]),
	}
	for i := range result.latencyHist {
		result.latencyHist[i] = parseRedisInt(values[latencyHistField(i)])
		result.ttftHist[i] = parseRedisInt(values[ttftHistField(i)])
	}
	return result
}

func latencyHistField(i int) string {
	return "lh" + strconv.Itoa(i)
}

func ttftHistField(i int) string {
	return "th" + strconv.Itoa(i)
}

func parseRedisInt(value string) int64 {
	if value == "" {
		return 0
	}
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

// encodeLatencyHist serialises the per-bucket counts for the text column.
func encodeLatencyHist(hist [latencyHistBuckets]int64) string {
	var b strings.Builder
	for i, n := range hist {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatInt(n, 10))
	}
	return b.String()
}

// decodeLatencyHist accepts anything the column may hold, including the empty
// string written before histograms existed, and returns all-zero buckets.
func decodeLatencyHist(raw string) [latencyHistBuckets]int64 {
	var hist [latencyHistBuckets]int64
	if raw == "" {
		return hist
	}
	i := 0
	for part := range strings.SplitSeq(raw, ",") {
		if i >= latencyHistBuckets {
			break
		}
		hist[i] = parseRedisInt(part)
		i++
	}
	return hist
}
