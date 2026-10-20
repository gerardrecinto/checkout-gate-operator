import test from 'node:test';
import assert from 'node:assert/strict';
import { evaluate } from './evaluate.js';

const spec = { maxErrorRateMilliPercent: 500, maxP99LatencyMs: 450, maxCpuSaturationPercent: 80 };
const ok = { errorRateMilliPercent: 120, p99LatencyMs: 200, cpuSaturationPercent: 40, readyReplicas: 3, desiredReplicas: 3 };

test('healthy target passes', () => {
  assert.deepEqual(evaluate(spec, ok), { verdict: 'Pass', message: 'all thresholds satisfied' });
});

test('error rate over the max breaches with the Go message format', () => {
  const r = evaluate(spec, { ...ok, errorRateMilliPercent: 2400 });
  assert.equal(r.verdict, 'Breach');
  assert.equal(r.message, 'error rate 2.400% exceeds max 0.500%');
});

test('latency over the max breaches', () => {
  const r = evaluate(spec, { ...ok, p99LatencyMs: 18094 });
  assert.deepEqual(r, { verdict: 'Breach', message: 'p99 latency 18094ms exceeds max 450ms' });
});

test('cpu saturation only warns', () => {
  const r = evaluate(spec, { ...ok, cpuSaturationPercent: 95 });
  assert.equal(r.verdict, 'Warn');
  assert.match(r.message, /^cpu saturation 95% exceeds max 80%/);
});

test('desired replicas with none ready breaches before any metric is read', () => {
  const r = evaluate(spec, { ...ok, readyReplicas: 0, errorRateMilliPercent: 0 });
  assert.deepEqual(r, { verdict: 'Breach', message: 'target has 3 desired replicas but 0 ready' });
});

test('values equal to a threshold pass', () => {
  const r = evaluate(spec, { ...ok, errorRateMilliPercent: 500, p99LatencyMs: 450, cpuSaturationPercent: 80 });
  assert.equal(r.verdict, 'Pass');
});

test('a breach outranks a warn', () => {
  const r = evaluate(spec, { ...ok, p99LatencyMs: 900, cpuSaturationPercent: 99 });
  assert.equal(r.verdict, 'Breach');
});

test('zero desired replicas is not a breach', () => {
  const r = evaluate(spec, { ...ok, desiredReplicas: 0, readyReplicas: 0 });
  assert.equal(r.verdict, 'Pass');
});
