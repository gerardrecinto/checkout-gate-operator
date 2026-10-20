package dev.gerardrecinto.verdict;

/** Same rules, in the same order, as gate.Evaluate in the Go operator. */
public final class GateEvaluator {
    private GateEvaluator() {}

    public static Result evaluate(Thresholds t, Metrics m) {
        if (m.desiredReplicas() > 0 && m.readyReplicas() == 0) {
            return new Result(Verdict.BREACH,
                    "target has %d desired replicas but 0 ready".formatted(m.desiredReplicas()));
        }
        if (m.errorRateMilliPercent() > t.maxErrorRateMilliPercent()) {
            return new Result(Verdict.BREACH, "error rate %.3f%% exceeds max %.3f%%".formatted(
                    m.errorRateMilliPercent() / 1000.0, t.maxErrorRateMilliPercent() / 1000.0));
        }
        if (m.p99LatencyMs() > t.maxP99LatencyMs()) {
            return new Result(Verdict.BREACH, "p99 latency %dms exceeds max %dms".formatted(
                    m.p99LatencyMs(), t.maxP99LatencyMs()));
        }
        if (m.cpuSaturationPercent() > t.maxCpuSaturationPercent()) {
            return new Result(Verdict.WARN, "cpu saturation %d%% exceeds max %d%%".formatted(
                    m.cpuSaturationPercent(), t.maxCpuSaturationPercent()));
        }
        return new Result(Verdict.PASS, "all thresholds satisfied");
    }
}
