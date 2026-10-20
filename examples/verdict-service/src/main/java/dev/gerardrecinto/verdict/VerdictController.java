package dev.gerardrecinto.verdict;

import io.micrometer.core.instrument.MeterRegistry;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

@RestController
class VerdictController {
    private final MeterRegistry registry;

    VerdictController(MeterRegistry registry) {
        this.registry = registry;
    }

    @PostMapping("/v1/verdict")
    Result evaluate(@Valid @RequestBody VerdictRequest request) {
        Result result = GateEvaluator.evaluate(request.thresholds().toDomain(), request.metrics().toDomain());
        registry.counter("verdict.evaluations", "verdict", result.verdict().name()).increment();
        return result;
    }
}
