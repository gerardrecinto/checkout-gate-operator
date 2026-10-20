package dev.gerardrecinto.verdict;

import jakarta.validation.Valid;
import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotNull;

public record VerdictRequest(@NotNull @Valid ThresholdsDto thresholds, @NotNull @Valid MetricsDto metrics) {

    public record ThresholdsDto(
            @Min(0) @Max(100000) int maxErrorRateMilliPercent,
            @Min(0) int maxP99LatencyMs,
            @Min(0) @Max(100) int maxCpuSaturationPercent) {
        Thresholds toDomain() {
            return new Thresholds(maxErrorRateMilliPercent, maxP99LatencyMs, maxCpuSaturationPercent);
        }
    }

    public record MetricsDto(
            @Min(0) int errorRateMilliPercent,
            @Min(0) int p99LatencyMs,
            @Min(0) @Max(100) int cpuSaturationPercent,
            @Min(0) int desiredReplicas,
            @Min(0) int readyReplicas) {
        Metrics toDomain() {
            return new Metrics(errorRateMilliPercent, p99LatencyMs, cpuSaturationPercent, desiredReplicas, readyReplicas);
        }
    }
}
