package dev.gerardrecinto.verdict;

public record Metrics(int errorRateMilliPercent, int p99LatencyMs, int cpuSaturationPercent,
                      int desiredReplicas, int readyReplicas) {}
