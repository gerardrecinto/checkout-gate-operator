package dev.gerardrecinto.verdict;

public record Thresholds(int maxErrorRateMilliPercent, int maxP99LatencyMs, int maxCpuSaturationPercent) {}
