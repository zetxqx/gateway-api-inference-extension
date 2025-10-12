/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metrics

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"go.uber.org/multierr"

	"sigs.k8s.io/gateway-api-inference-extension/pkg/epp/backend"
)

const (
	// LoRA metrics based on protocol
	LoraInfoRunningAdaptersMetricName = "running_lora_adapters"
	LoraInfoWaitingAdaptersMetricName = "waiting_lora_adapters"
	LoraInfoMaxAdaptersMetricName     = "max_lora"
)

type PodMetricsClientImpl struct {
	MetricMapping            *MetricMapping
	ModelServerMetricsPort   int32
	ModelServerMetricsPath   string
	ModelServerMetricsScheme string

	Client *http.Client
}

// FetchMetrics fetches metrics from a given pod, clones the existing metrics object and returns an updated one.
func (p *PodMetricsClientImpl) FetchMetrics(ctx context.Context, pod *backend.Pod, existing *MetricsState, port int32) (*MetricsState, error) {
	url := p.getMetricEndpoint(pod, port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metrics from %s: %w", pod.NamespacedName, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from %s: %v", pod.NamespacedName, resp.StatusCode)
	}

	parser := expfmt.NewTextParser(model.LegacyValidation)
	metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, err
	}
	return p.promToPodMetrics(metricFamilies, existing)
}

func (p *PodMetricsClientImpl) getMetricEndpoint(pod *backend.Pod, targetPortNumber int32) string {
	if p.ModelServerMetricsPort == 0 {
		p.ModelServerMetricsPort = targetPortNumber
	}
	return fmt.Sprintf("%s://%s:%d%s", p.ModelServerMetricsScheme, pod.Address, p.ModelServerMetricsPort, p.ModelServerMetricsPath)
}

// promToPodMetrics updates internal pod metrics with scraped Prometheus metrics.
func (p *PodMetricsClientImpl) promToPodMetrics(
	metricFamilies map[string]*dto.MetricFamily,
	existing *MetricsState,
) (*MetricsState, error) {
	var errs error
	updated := existing.Clone()

	if p.MetricMapping.TotalQueuedRequests != nil {
		queued, err := p.getMetric(metricFamilies, *p.MetricMapping.TotalQueuedRequests)
		if err == nil {
			updated.WaitingQueueSize = int(queued.GetGauge().GetValue())
		} else {
			errs = multierr.Append(errs, err)
		}
	}

	if p.MetricMapping.KVCacheUtilization != nil {
		usage, err := p.getMetric(metricFamilies, *p.MetricMapping.KVCacheUtilization)
		if err == nil {
			updated.KVCacheUsagePercent = usage.GetGauge().GetValue()
		} else {
			errs = multierr.Append(errs, err)
		}
	}

	// Handle LoRA metrics (only if all LoRA MetricSpecs are present)
	if p.MetricMapping.LoraRequestInfo != nil {
		loraMetrics, err := p.getLatestLoraMetric(metricFamilies)
		errs = multierr.Append(errs, err)

		if loraMetrics != nil {
			updated.ActiveModels = make(map[string]int)
			updated.WaitingModels = make(map[string]int)
			for _, label := range loraMetrics.GetLabel() {
				if label.GetName() == LoraInfoRunningAdaptersMetricName {
					if label.GetValue() != "" {
						adapterList := strings.Split(label.GetValue(), ",")
						for _, adapter := range adapterList {
							updated.ActiveModels[adapter] = 0
						}
					}
				}
				if label.GetName() == LoraInfoWaitingAdaptersMetricName {
					if label.GetValue() != "" {
						adapterList := strings.Split(label.GetValue(), ",")
						for _, adapter := range adapterList {
							updated.WaitingModels[adapter] = 0
						}
					}
				}
				if label.GetName() == LoraInfoMaxAdaptersMetricName {
					if label.GetValue() != "" {
						updated.MaxActiveModels, err = strconv.Atoi(label.GetValue())
						if err != nil {
							errs = multierr.Append(errs, err)
						}
					}
				}
			}
		}
	}

	if p.MetricMapping.CacheConifgInfo != nil {
		cacheConfigInfo, err := p.getCacheConfigInfo(metricFamilies)
		if err != nil {
			errs = multierr.Append(errs, err)
		} else if cacheConfigInfo != nil {
			updated.BlockSize = cacheConfigInfo.blockSize
			updated.NumGPUBlocks = cacheConfigInfo.numGPUBlocks
		}
	}

	return updated, errs
}

// getLatestLoraMetric gets latest lora metric series in gauge metric family `vllm:lora_requests_info`
// reason its specially fetched is because each label key value pair permutation generates new series
// and only most recent is useful. The value of each series is the creation timestamp so we can
// retrieve the latest by sorting the value.
func (p *PodMetricsClientImpl) getLatestLoraMetric(metricFamilies map[string]*dto.MetricFamily) (*dto.Metric, error) {
	if p.MetricMapping.LoraRequestInfo == nil {
		return nil, nil // No LoRA metrics configured
	}

	loraRequests, ok := metricFamilies[p.MetricMapping.LoraRequestInfo.MetricName]
	if !ok {
		return nil, fmt.Errorf("metric family %q not found", p.MetricMapping.LoraRequestInfo.MetricName)
	}

	var latest *dto.Metric
	var latestTs float64 // Use float64, as Gauge.Value is float64

	// Iterate over all metrics in the family.
	for _, m := range loraRequests.GetMetric() {
		running := ""
		waiting := ""
		// Check if the metric has the expected LoRA labels.
		for _, lp := range m.GetLabel() {
			switch lp.GetName() {
			case LoraInfoRunningAdaptersMetricName:
				running = lp.GetValue()
			case LoraInfoWaitingAdaptersMetricName:
				waiting = lp.GetValue()
			}
		}
		// Ignore metrics with both labels empty.
		if running == "" && waiting == "" {
			continue
		}

		// Select the metric with the *largest Gauge Value* (which represents the timestamp).
		if m.GetGauge().GetValue() > latestTs {
			latestTs = m.GetGauge().GetValue()
			latest = m
		}
	}
	if latest == nil {
		return nil, nil
	}

	return latest, nil // Convert nanoseconds to time.Time
}

type cacheInfo struct {
	blockSize    int
	numGPUBlocks int
}

func (p *PodMetricsClientImpl) getCacheConfigInfo(metricFamilies map[string]*dto.MetricFamily) (*cacheInfo, error) {
	if p.MetricMapping.CacheConifgInfo == nil {
		return nil, nil // No CacheConfigInfo configured.
	}

	metricName := p.MetricMapping.CacheConifgInfo.MetricName
	cacheConfigInfo, ok := metricFamilies[metricName]
	if !ok {
		return nil, fmt.Errorf("metric family %q not found", p.MetricMapping.CacheConifgInfo.MetricName)
	}

	if len(cacheConfigInfo.GetMetric()) == 0 {
		return nil, fmt.Errorf("no metrics found in family %q", metricName)
	}

	var blockSizeStr, numGPUBlocksStr string
	for _, label := range cacheConfigInfo.GetMetric()[0].GetLabel() {
		switch label.GetName() {
		case "block_size":
			blockSizeStr = label.GetValue()
		case "num_gpu_blocks":
			numGPUBlocksStr = label.GetValue()
		}
	}

	var errs error
	blockSize, err := strconv.Atoi(blockSizeStr)
	if err != nil {
		errs = multierr.Append(errs, err)
	}
	numGPUBlocks, err := strconv.Atoi(numGPUBlocksStr)
	if err != nil {
		errs = multierr.Append(errs, err)
	}
	if errs != nil {
		return nil, errs
	}
	return &cacheInfo{blockSize, numGPUBlocks}, nil
}

// getMetric retrieves a specific metric based on MetricSpec.
func (p *PodMetricsClientImpl) getMetric(metricFamilies map[string]*dto.MetricFamily, spec MetricSpec) (*dto.Metric, error) {
	mf, ok := metricFamilies[spec.MetricName]
	if !ok {
		return nil, fmt.Errorf("metric family %q not found", spec.MetricName)
	}

	if len(mf.GetMetric()) == 0 {
		return nil, fmt.Errorf("no metrics available for %q", spec.MetricName)
	}

	return getLatestMetric(mf, &spec)
}

// getLabeledMetric gets the latest metric with matching labels.
func getLatestMetric(mf *dto.MetricFamily, spec *MetricSpec) (*dto.Metric, error) {
	var latestMetric *dto.Metric
	var latestTimestamp int64 = -1 // Initialize to -1 so any timestamp is greater

	for _, m := range mf.GetMetric() {
		if spec.Labels == nil || labelsMatch(m.GetLabel(), spec.Labels) {
			if m.GetTimestampMs() > latestTimestamp {
				latestTimestamp = m.GetTimestampMs()
				latestMetric = m
			}
		}
	}

	if latestMetric != nil {
		return latestMetric, nil
	}

	return nil, fmt.Errorf("no matching metric found for %q with labels %+v", spec.MetricName, spec.Labels)
}

// labelsMatch checks if a metric's labels contain all the labels in the spec.
func labelsMatch(metricLabels []*dto.LabelPair, specLabels map[string]string) bool {
	if len(specLabels) == 0 {
		return true // No specific labels required
	}

	for specName, specValue := range specLabels {
		found := false
		for _, label := range metricLabels {
			if label.GetName() == specName && label.GetValue() == specValue {
				found = true
				break
			}
		}
		if !found {
			return false // A required label is missing
		}
	}
	return true // All required labels are present
}
