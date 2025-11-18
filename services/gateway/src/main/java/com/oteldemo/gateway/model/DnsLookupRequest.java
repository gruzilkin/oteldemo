package com.oteldemo.gateway.model;

import com.fasterxml.jackson.annotation.JsonProperty;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.List;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class DnsLookupRequest {

    @JsonProperty("domain")
    private String domain;

    @JsonProperty("locations")
    private List<String> locations;

    @JsonProperty("record_types")
    private List<String> recordTypes;
}
