package com.oteldemo.gateway.model;

import com.fasterxml.jackson.annotation.JsonProperty;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.Map;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class DnsLookupResponse {

    @JsonProperty("request_id")
    private String requestId;

    @JsonProperty("domain")
    private String domain;

    @JsonProperty("status")
    private String status;

    @JsonProperty("results")
    private Map<String, Object> results;

    @JsonProperty("message")
    private String message;
}
