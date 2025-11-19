package com.oteldemo.gateway.service;

import com.oteldemo.gateway.model.DnsLookupRequest;
import com.oteldemo.gateway.model.DnsLookupResponse;
import io.opentelemetry.api.trace.Span;
import io.opentelemetry.api.trace.StatusCode;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.*;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestTemplate;

import java.util.HashMap;
import java.util.Map;

@Service
public class OrchestratorService {

    private static final Logger logger = LoggerFactory.getLogger(OrchestratorService.class);

    @Autowired
    private RestTemplate restTemplate;

    @Value("${orchestrator.url:http://orchestrator:8001}")
    private String orchestratorUrl;

    public DnsLookupResponse submitDnsLookup(DnsLookupRequest request) {
        String url = orchestratorUrl + "/api/v1/dns/orchestrate";

        logger.info("Forwarding DNS lookup to orchestrator: {}", url);

        Span currentSpan = Span.current();
        currentSpan.setAttribute("orchestrator.url", url);
        String traceId = currentSpan.getSpanContext().getTraceId();

        try {
            // Prepare request body (trace context propagated via HTTP headers automatically)
            Map<String, Object> requestBody = new HashMap<>();
            requestBody.put("domain", request.getDomain());
            requestBody.put("locations", request.getLocations());
            requestBody.put("record_types", request.getRecordTypes());

            // Set headers
            HttpHeaders headers = new HttpHeaders();
            headers.setContentType(MediaType.APPLICATION_JSON);

            HttpEntity<Map<String, Object>> entity = new HttpEntity<>(requestBody, headers);

            // Call orchestrator
            ResponseEntity<DnsLookupResponse> response = restTemplate.exchange(
                url,
                HttpMethod.POST,
                entity,
                DnsLookupResponse.class
            );

            if (response.getStatusCode().is2xxSuccessful() && response.getBody() != null) {
                logger.info("Successfully received response from orchestrator");
                return response.getBody();
            } else {
                logger.warn("Orchestrator returned non-success status: {}", response.getStatusCode());
                return new DnsLookupResponse(
                    request.getDomain(),
                    "error",
                    null,
                    "Orchestrator returned status: " + response.getStatusCode()
                );
            }

        } catch (Exception e) {
            logger.error("Error communicating with orchestrator: {}", e.getMessage(), e);
            currentSpan.recordException(e);
            currentSpan.setStatus(StatusCode.ERROR, "Failed to communicate with orchestrator");

            return new DnsLookupResponse(
                request.getDomain(),
                "error",
                null,
                "Failed to communicate with orchestrator: " + e.getMessage()
            );
        }
    }
}
