# Helm Provider Configuration Options

This document outlines three potential patterns for configuring provider-specific values in our Helm chart, using the `gke.autopilot` flag as the primary example. The goal is to find a solution that is clear, scalable, and easy for users to manage.

---

## Option 1: Keep As Is (Nested under `monitoring`)

This is the current approach, where the GKE-specific setting is nested directly under the feature it affects.

### `values.yaml` Snippet
```yaml
# ...
monitoring:
  interval: "10s"
  # ...
  gke:
    enabled: false
    # Set to true if the cluster is an Autopilot cluster.
    autopilot: false
# ...
```

### Template Logic Snippet (`gke.yaml`)
The template logic directly accesses this nested value.
```go
{{- if .Values.monitoring.gke.enabled }}
  {{- $gmpNamespace := "gmp-system" -}}
  {{- if .Values.monitoring.gke.autopilot -}}
  {{-   $gmpNamespace = "gke-gmp-system" -}}
  {{- end -}}
  {{/* ... rest of logic ... */}}
{{- end }}
```

### Pros & Cons
- **Pros:**
  - **Simple & Direct:** The configuration is located right next to the feature it modifies (`monitoring`).
  - **Low Complexity:** No advanced Helm logic is needed.
- **Cons:**
  - **Not Scalable:** If we add another GKE-specific setting that isn't related to monitoring (e.g., a special annotation), the provider-specific settings become scattered across `values.yaml`.
  - **Poor Discoverability:** It's not immediately obvious that the chart has GKE-specific options.

---

## Option 2: Centralized Provider with a `name` field

This approach uses a `name` field to explicitly declare the provider, with provider-specific configurations in nested objects.

### `values.yaml` Snippet
```yaml
provider:
  # The name of the provider. Supported values: "gke", "none".
  name: gke

  # GKE-specific configuration.
  # This block is only used if name is "gke".
  gke:
    # Set to true if the cluster is an Autopilot cluster.
    autopilot: false
```

### Template Logic Snippet (`gke.yaml`)
The logic first checks the `name` field, then accesses the nested configuration.
```go
{{- if eq .Values.provider.name "gke" }}
  {{- $gmpNamespace := "gmp-system" -}}
  {{- $isAutopilot := .Values.provider.gke.autopilot | default false -}}
  {{- if $isAutopilot -}}
  {{-   $gmpNamespace = "gke-gmp-system" -}}
  {{- end -}}
  {{/* ... rest of logic ... */}}
{{- end }}
```

### Pros & Cons
- **Pros:**
  - **Explicit:** The active provider is clearly declared.
  - **Centralized:** All provider settings are grouped under a single `provider` key.
- **Cons:**
  - **Verbose:** Requires both a `name` and a configuration block.
  - **Prone to Error:** A user could set `name: gke` but forget to configure the `gke` block, or vice-versa. The validation logic is more complex.

---

## Option 3: Exclusive Provider Block (Idiomatic `oneOf` Pattern)

This is a common and robust Helm pattern. The presence of a provider's configuration block acts as the switch to enable it. This enforces a "one-of" choice among providers.

### `values.yaml` Snippet
The user enables a provider by uncommenting its block. Only one block should be active.
```yaml
# Cloud provider specific configuration.
# You MUST enable exactly ONE provider.
provider:
  # Google Kubernetes Engine (GKE) specific configuration
  gke:
    # Set to true if the cluster is an Autopilot cluster.
    # This is optional and defaults to false if not set.
    autopilot: false

  # Example for another provider (would be commented out)
  # aws:
  #   region: us-east-1

  # Generic provider for non-cloud-specific setups (would be commented out)
  # none: {}
```

### Template Logic Snippet

A validation template (`_validations.tpl`) ensures exactly one provider is configured.
```go
{{/* Validate that exactly one provider is configured */}}
{{- define "inferencepool.validate.provider" -}}
{{- $definedProviders := keys .Values.provider | sortAlpha -}}
{{- if ne (len $definedProviders) 1 -}}
{{-   $msg := printf "Invalid provider configuration: Exactly one provider must be configured under the 'provider' key. Found %d: %s" (len $definedProviders) (join ", " $definedProviders) -}}
{{-   fail $msg -}}
{{- end -}}
{{- end -}}
```
The logic in `gke.yaml` becomes very clean and handles defaults gracefully.
```go
{{/* This validation must be included at the top of a main template */}}
{{- include "inferencepool.validate.provider" . -}}

{{/* Logic in gke.yaml */}}
{{- if .Values.provider.gke -}}
  {{- $gmpNamespace := "gmp-system" -}}
  {{- $isAutopilot := .Values.provider.gke.autopilot | default false -}}
  {{- if $isAutopilot -}}
  {{-   $gmpNamespace = "gke-gmp-system" -}}
  {{- end -}}
  {{/* ... rest of logic ... */}}
{{- end }}
```

### Pros & Cons
- **Pros:**
  - **Idiomatic & Clean:** This is a standard, best-practice pattern in the Helm community.
  - **Self-Validating Structure:** The structure itself prevents misconfiguration. The `fail` logic provides clear errors to the user.
  - **Scalable:** Easy to add new providers (`aws`, `azure`) with their own specific, cleanly-nested options.
  - **Handles Defaults Gracefully:** Optional values like `autopilot` are easily managed with `| default false`.
- **Cons:**
  - **Slightly Higher Initial Complexity:** Requires creating a validation template.
  - **Relies on User Convention:** The user must understand the pattern of uncommenting one provider block.
