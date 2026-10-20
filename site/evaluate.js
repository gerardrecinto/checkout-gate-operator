// Port of internal/gate/evaluator.go. Same checks in the same order, same messages.
export function evaluate(spec, m) {
  if (m.desiredReplicas > 0 && m.readyReplicas === 0) {
    return { verdict: 'Breach', message: `target has ${m.desiredReplicas} desired replicas but 0 ready` };
  }
  if (m.errorRateMilliPercent > spec.maxErrorRateMilliPercent) {
    return {
      verdict: 'Breach',
      message: `error rate ${(m.errorRateMilliPercent / 1000).toFixed(3)}% exceeds max ${(spec.maxErrorRateMilliPercent / 1000).toFixed(3)}%`,
    };
  }
  if (m.p99LatencyMs > spec.maxP99LatencyMs) {
    return { verdict: 'Breach', message: `p99 latency ${m.p99LatencyMs}ms exceeds max ${spec.maxP99LatencyMs}ms` };
  }
  if (m.cpuSaturationPercent > spec.maxCpuSaturationPercent) {
    return {
      verdict: 'Warn',
      message: `cpu saturation ${m.cpuSaturationPercent}% exceeds max ${spec.maxCpuSaturationPercent}%, holding rather than breaching on a resource signal alone`,
    };
  }
  return { verdict: 'Pass', message: 'all thresholds satisfied' };
}
