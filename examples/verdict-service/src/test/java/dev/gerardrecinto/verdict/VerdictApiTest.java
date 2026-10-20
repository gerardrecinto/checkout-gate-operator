package dev.gerardrecinto.verdict;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.content;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.actuate.observability.AutoConfigureObservability;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;

@SpringBootTest
@AutoConfigureMockMvc
@AutoConfigureObservability
class VerdictApiTest {
    @Autowired
    MockMvc mvc;

    private static final String HEALTHY = """
        {"thresholds":{"maxErrorRateMilliPercent":500,"maxP99LatencyMs":450,"maxCpuSaturationPercent":80},
         "metrics":{"errorRateMilliPercent":100,"p99LatencyMs":200,"cpuSaturationPercent":40,
                    "desiredReplicas":3,"readyReplicas":3}}""";

    @Test
    void evaluateReturnsPassForHealthyMetrics() throws Exception {
        mvc.perform(post("/v1/verdict").contentType(MediaType.APPLICATION_JSON).content(HEALTHY))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.verdict").value("PASS"));
    }

    @Test
    void evaluateRejectsAMissingMetricsBlock() throws Exception {
        mvc.perform(post("/v1/verdict").contentType(MediaType.APPLICATION_JSON)
                        .content("{\"thresholds\":{\"maxErrorRateMilliPercent\":1,\"maxP99LatencyMs\":1,\"maxCpuSaturationPercent\":1}}"))
                .andExpect(status().isBadRequest());
    }

    @Test
    void evaluateRejectsACpuLimitOverOneHundred() throws Exception {
        String bad = HEALTHY.replace("\"maxCpuSaturationPercent\":80", "\"maxCpuSaturationPercent\":101");
        mvc.perform(post("/v1/verdict").contentType(MediaType.APPLICATION_JSON).content(bad))
                .andExpect(status().isBadRequest());
    }

    @Test
    void livenessAndReadinessProbesAreUp() throws Exception {
        mvc.perform(get("/actuator/health/liveness")).andExpect(status().isOk())
                .andExpect(jsonPath("$.status").value("UP"));
        mvc.perform(get("/actuator/health/readiness")).andExpect(status().isOk());
    }

    @Test
    void prometheusEndpointCountsVerdicts() throws Exception {
        mvc.perform(post("/v1/verdict").contentType(MediaType.APPLICATION_JSON).content(HEALTHY));
        mvc.perform(get("/actuator/prometheus")).andExpect(status().isOk())
                .andExpect(content().string(org.hamcrest.Matchers.containsString("verdict_evaluations_total{verdict=\"PASS\"")));
    }

    @Test
    void unexposedActuatorEndpointsAreNotAvailable() throws Exception {
        mvc.perform(get("/actuator/env")).andExpect(status().isNotFound());
    }
}
