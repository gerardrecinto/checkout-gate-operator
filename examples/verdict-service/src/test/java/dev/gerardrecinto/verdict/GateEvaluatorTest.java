package dev.gerardrecinto.verdict;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class GateEvaluatorTest {
    private final Thresholds limits = new Thresholds(500, 450, 80);

    private Result eval(int errMilli, int p99, int cpu, int desired, int ready) {
        return GateEvaluator.evaluate(limits, new Metrics(errMilli, p99, cpu, desired, ready));
    }

    @Test
    void passesWhenEverythingIsUnderTheLimits() {
        assertEquals(Verdict.PASS, eval(100, 200, 40, 3, 3).verdict());
    }

    @Test
    void breachesWhenNoReplicaIsReady() {
        Result r = eval(0, 0, 0, 3, 0);
        assertEquals(Verdict.BREACH, r.verdict());
        assertTrue(r.message().contains("0 ready"));
    }

    @Test
    void breachesOnErrorRate() {
        assertEquals(Verdict.BREACH, eval(501, 200, 40, 3, 3).verdict());
    }

    @Test
    void breachesOnP99Latency() {
        assertEquals(Verdict.BREACH, eval(100, 451, 40, 3, 3).verdict());
    }

    @Test
    void warnsOnCpuAloneInsteadOfBreaching() {
        assertEquals(Verdict.WARN, eval(100, 200, 81, 3, 3).verdict());
    }

    @Test
    void valuesExactlyAtTheLimitStillPass() {
        assertEquals(Verdict.PASS, eval(500, 450, 80, 3, 3).verdict());
    }

    @Test
    void aBreachOutranksACpuWarning() {
        assertEquals(Verdict.BREACH, eval(501, 200, 99, 3, 3).verdict());
    }
}
