package com.oteldemo.gateway.controller;

import com.oteldemo.gateway.model.DnsLookupRequest;
import com.oteldemo.gateway.model.DnsLookupResponse;
import com.oteldemo.gateway.service.OrchestratorService;
import io.opentelemetry.api.trace.Span;
import io.opentelemetry.api.trace.Tracer;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.Arrays;
import java.util.UUID;

@RestController
@RequestMapping("/api/v1")
public class DnsLookupController {

    private static final Logger logger = LoggerFactory.getLogger(DnsLookupController.class);

    @Autowired
    private OrchestratorService orchestratorService;

    @PostMapping("/dns/lookup")
    public ResponseEntity<DnsLookupResponse> lookupDns(@RequestBody DnsLookupRequest request) {
        String requestId = UUID.randomUUID().toString();

        logger.info("Received DNS lookup request for domain: {} with requestId: {}",
                    request.getDomain(), requestId);

        // Add current span attributes
        Span currentSpan = Span.current();
        currentSpan.setAttribute("dns.domain", request.getDomain());
        currentSpan.setAttribute("dns.locations", String.join(",", request.getLocations()));
        currentSpan.setAttribute("request.id", requestId);

        try {
            // Validate request
            if (request.getDomain() == null || request.getDomain().isEmpty()) {
                return ResponseEntity.badRequest().body(
                    new DnsLookupResponse(requestId, null, "error", null, "Domain is required")
                );
            }

            // Set default locations if not provided
            if (request.getLocations() == null || request.getLocations().isEmpty()) {
                request.setLocations(Arrays.asList("us-east-1", "eu-west-1", "asia-south-1"));
            }

            // Set default record types if not provided
            if (request.getRecordTypes() == null || request.getRecordTypes().isEmpty()) {
                request.setRecordTypes(Arrays.asList("A", "AAAA", "MX", "TXT", "NS"));
            }

            // Forward to orchestrator
            DnsLookupResponse response = orchestratorService.submitDnsLookup(requestId, request);

            logger.info("DNS lookup request {} processed successfully", requestId);
            currentSpan.setAttribute("response.status", response.getStatus());

            return ResponseEntity.ok(response);

        } catch (Exception e) {
            logger.error("Error processing DNS lookup request {}: {}", requestId, e.getMessage(), e);
            currentSpan.setAttribute("error", true);
            currentSpan.setAttribute("error.message", e.getMessage());

            return ResponseEntity.status(HttpStatus.INTERNAL_SERVER_ERROR).body(
                new DnsLookupResponse(requestId, request.getDomain(), "error", null,
                                     "Internal server error: " + e.getMessage())
            );
        }
    }

    @GetMapping("/health")
    public ResponseEntity<String> health() {
        return ResponseEntity.ok("Gateway is healthy");
    }
}
