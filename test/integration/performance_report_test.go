package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PerformanceReport contains comprehensive test results
type PerformanceReport struct {
	TestSuite     string                    `json:"test_suite"`
	ExecutionTime time.Time                 `json:"execution_time"`
	Environment   EnvironmentInfo           `json:"environment"`
	Summary       TestSummary               `json:"summary"`
	Tests         []TestResult              `json:"tests"`
	Conclusions   []string                  `json:"conclusions"`
	Recommendations []string                `json:"recommendations"`
}

// EnvironmentInfo captures test environment details
type EnvironmentInfo struct {
	DRAAddress     string `json:"dra_address"`
	EIRAddress     string `json:"eir_address"`
	GatewayAddress string `json:"gateway_address"`
	GoVersion      string `json:"go_version"`
	TestDuration   string `json:"test_duration"`
}

// TestSummary provides high-level metrics
type TestSummary struct {
	TotalTests       int     `json:"total_tests"`
	PassedTests      int     `json:"passed_tests"`
	FailedTests      int     `json:"failed_tests"`
	TotalRequests    int64   `json:"total_requests"`
	TotalSuccess     int64   `json:"total_success"`
	TotalFailures    int64   `json:"total_failures"`
	OverallSuccessRate float64 `json:"overall_success_rate"`
	MaxThroughput    float64 `json:"max_throughput_rps"`
	BestP95Latency   int64   `json:"best_p95_latency_ms"`
	WorstP95Latency  int64   `json:"worst_p95_latency_ms"`
}

// TestResult captures individual test metrics
type TestResult struct {
	Name              string              `json:"name"`
	Category          string              `json:"category"`
	Status            string              `json:"status"`
	Metrics           *PerformanceMetrics `json:"metrics"`
	Configuration     TestConfiguration   `json:"configuration"`
	Observations      []string            `json:"observations"`
	PerformanceGrade  string              `json:"performance_grade"`
}

// TestConfiguration captures test parameters
type TestConfiguration struct {
	Concurrency       int           `json:"concurrency"`
	RequestsPerClient int           `json:"requests_per_client"`
	Duration          time.Duration `json:"duration,omitempty"`
	TargetRPS         float64       `json:"target_rps,omitempty"`
}

// PerformanceReportGenerator generates comprehensive performance reports
type PerformanceReportGenerator struct {
	report *PerformanceReport
}

// NewPerformanceReportGenerator creates a new report generator
func NewPerformanceReportGenerator() *PerformanceReportGenerator {
	return &PerformanceReportGenerator{
		report: &PerformanceReport{
			TestSuite:     "EIR S13 Interface Performance Test Suite",
			ExecutionTime: time.Now(),
			Environment: EnvironmentInfo{
				DRAAddress:     "localhost:3869",
				EIRAddress:     "localhost:8080",
				GatewayAddress: "localhost:3868",
				GoVersion:      "1.25.3",
			},
			Tests:           make([]TestResult, 0),
			Conclusions:     make([]string, 0),
			Recommendations: make([]string, 0),
		},
	}
}

// AddTestResult adds a test result to the report
func (prg *PerformanceReportGenerator) AddTestResult(result TestResult) {
	prg.report.Tests = append(prg.report.Tests, result)
}

// GenerateSummary calculates summary metrics from all tests
func (prg *PerformanceReportGenerator) GenerateSummary() {
	summary := &prg.report.Summary

	summary.TotalTests = len(prg.report.Tests)

	var maxThroughput float64
	var bestP95 int64 = 999999
	var worstP95 int64 = 0

	for _, test := range prg.report.Tests {
		if test.Status == "PASS" {
			summary.PassedTests++
		} else {
			summary.FailedTests++
		}

		if test.Metrics != nil {
			summary.TotalRequests += test.Metrics.TotalRequests
			summary.TotalSuccess += test.Metrics.SuccessfulReqs
			summary.TotalFailures += test.Metrics.FailedReqs

			if test.Metrics.RequestsPerSecond > maxThroughput {
				maxThroughput = test.Metrics.RequestsPerSecond
			}

			if test.Metrics.P95LatencyMs < bestP95 && test.Metrics.P95LatencyMs > 0 {
				bestP95 = test.Metrics.P95LatencyMs
			}

			if test.Metrics.P95LatencyMs > worstP95 {
				worstP95 = test.Metrics.P95LatencyMs
			}
		}
	}

	if summary.TotalRequests > 0 {
		summary.OverallSuccessRate = float64(summary.TotalSuccess) / float64(summary.TotalRequests) * 100
	}

	summary.MaxThroughput = maxThroughput
	summary.BestP95Latency = bestP95
	summary.WorstP95Latency = worstP95
}

// AddConclusions generates conclusions based on test results
func (prg *PerformanceReportGenerator) AddConclusions() {
	summary := &prg.report.Summary

	// Success rate analysis
	if summary.OverallSuccessRate >= 99.9 {
		prg.report.Conclusions = append(prg.report.Conclusions,
			"✓ Excellent reliability: >99.9% success rate achieved across all tests")
	} else if summary.OverallSuccessRate >= 99.0 {
		prg.report.Conclusions = append(prg.report.Conclusions,
			"✓ Good reliability: >99% success rate, suitable for production")
	} else {
		prg.report.Conclusions = append(prg.report.Conclusions,
			fmt.Sprintf("⚠ Reliability concern: %.2f%% success rate below 99%% threshold", summary.OverallSuccessRate))
	}

	// Throughput analysis
	if summary.MaxThroughput >= 1000 {
		prg.report.Conclusions = append(prg.report.Conclusions,
			fmt.Sprintf("✓ High throughput capacity: %.0f req/sec maximum", summary.MaxThroughput))
	} else if summary.MaxThroughput >= 500 {
		prg.report.Conclusions = append(prg.report.Conclusions,
			fmt.Sprintf("✓ Adequate throughput: %.0f req/sec for moderate load", summary.MaxThroughput))
	} else {
		prg.report.Conclusions = append(prg.report.Conclusions,
			fmt.Sprintf("⚠ Limited throughput: %.0f req/sec may need optimization", summary.MaxThroughput))
	}

	// Latency analysis
	if summary.BestP95Latency <= 50 {
		prg.report.Conclusions = append(prg.report.Conclusions,
			fmt.Sprintf("✓ Excellent latency: P95 = %d ms under optimal conditions", summary.BestP95Latency))
	} else if summary.BestP95Latency <= 100 {
		prg.report.Conclusions = append(prg.report.Conclusions,
			fmt.Sprintf("✓ Good latency: P95 = %d ms meets typical SLAs", summary.BestP95Latency))
	} else {
		prg.report.Conclusions = append(prg.report.Conclusions,
			fmt.Sprintf("⚠ Elevated latency: P95 = %d ms may impact user experience", summary.BestP95Latency))
	}

	// Stress test analysis
	if summary.WorstP95Latency > 500 {
		prg.report.Conclusions = append(prg.report.Conclusions,
			fmt.Sprintf("⚠ Performance degradation: P95 reached %d ms under stress", summary.WorstP95Latency))
	}
}

// AddRecommendations generates recommendations based on findings
func (prg *PerformanceReportGenerator) AddRecommendations() {
	summary := &prg.report.Summary

	if summary.OverallSuccessRate < 99.0 {
		prg.report.Recommendations = append(prg.report.Recommendations,
			"• Investigate failure causes and implement retry mechanisms")
	}

	if summary.MaxThroughput < 500 {
		prg.report.Recommendations = append(prg.report.Recommendations,
			"• Consider horizontal scaling or connection pooling optimization")
	}

	if summary.WorstP95Latency > 200 {
		prg.report.Recommendations = append(prg.report.Recommendations,
			"• Implement rate limiting to prevent latency spikes under heavy load",
			"• Review timeout configurations and optimize message processing")
	}

	if summary.BestP95Latency > 100 {
		prg.report.Recommendations = append(prg.report.Recommendations,
			"• Profile application to identify latency bottlenecks",
			"• Consider caching strategies for frequently accessed data")
	}

	// Always include general recommendations
	prg.report.Recommendations = append(prg.report.Recommendations,
		"• Monitor P95/P99 latencies in production to detect degradation early",
		"• Establish alerting thresholds based on observed baseline performance",
		"• Conduct regular load testing before major deployments")
}

// SaveJSON saves the report as JSON
func (prg *PerformanceReportGenerator) SaveJSON(filename string) error {
	data, err := json.MarshalIndent(prg.report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	dir := filepath.Dir(filename)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	return os.WriteFile(filename, data, 0644)
}

// SaveMarkdown saves the report as Markdown
func (prg *PerformanceReportGenerator) SaveMarkdown(filename string) error {
	md := prg.generateMarkdown()

	dir := filepath.Dir(filename)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	return os.WriteFile(filename, []byte(md), 0644)
}

// generateMarkdown generates a markdown report
func (prg *PerformanceReportGenerator) generateMarkdown() string {
	r := prg.report

	md := fmt.Sprintf("# %s\n\n", r.TestSuite)
	md += fmt.Sprintf("**Execution Date**: %s\n\n", r.ExecutionTime.Format("2006-01-02 15:04:05"))

	// Executive Summary
	md += "## Executive Summary\n\n"
	md += "| Metric | Value |\n"
	md += "|--------|-------|\n"
	md += fmt.Sprintf("| Total Tests | %d |\n", r.Summary.TotalTests)
	if r.Summary.TotalTests > 0 {
		md += fmt.Sprintf("| Passed | %d (%.1f%%) |\n", r.Summary.PassedTests,
			float64(r.Summary.PassedTests)/float64(r.Summary.TotalTests)*100)
	} else {
		md += fmt.Sprintf("| Passed | %d (0.0%%) |\n", r.Summary.PassedTests)
	}
	md += fmt.Sprintf("| Total Requests | %d |\n", r.Summary.TotalRequests)
	md += fmt.Sprintf("| Success Rate | %.2f%% |\n", r.Summary.OverallSuccessRate)
	md += fmt.Sprintf("| Max Throughput | %.0f req/sec |\n", r.Summary.MaxThroughput)
	md += fmt.Sprintf("| Best P95 Latency | %d ms |\n", r.Summary.BestP95Latency)
	md += fmt.Sprintf("| Worst P95 Latency | %d ms |\n\n", r.Summary.WorstP95Latency)

	// Test Environment
	md += "## Test Environment\n\n"
	md += "| Component | Address |\n"
	md += "|-----------|----------|\n"
	md += fmt.Sprintf("| DRA | %s |\n", r.Environment.DRAAddress)
	md += fmt.Sprintf("| EIR Core | %s |\n", r.Environment.EIRAddress)
	md += fmt.Sprintf("| Gateway | %s |\n", r.Environment.GatewayAddress)
	md += fmt.Sprintf("| Go Version | %s |\n\n", r.Environment.GoVersion)

	// Detailed Test Results
	md += "## Detailed Test Results\n\n"

	// Group tests by category
	categories := make(map[string][]TestResult)
	for _, test := range r.Tests {
		categories[test.Category] = append(categories[test.Category], test)
	}

	for category, tests := range categories {
		md += fmt.Sprintf("### %s\n\n", category)

		for _, test := range tests {
			md += fmt.Sprintf("#### %s\n\n", test.Name)
			md += fmt.Sprintf("**Status**: %s | **Grade**: %s\n\n", test.Status, test.PerformanceGrade)

			if test.Metrics != nil {
				md += "**Metrics:**\n\n"
				md += "| Metric | Value |\n"
				md += "|--------|-------|\n"
				md += fmt.Sprintf("| Total Requests | %d |\n", test.Metrics.TotalRequests)
				if test.Metrics.TotalRequests > 0 {
					md += fmt.Sprintf("| Success Rate | %.2f%% |\n",
						float64(test.Metrics.SuccessfulReqs)/float64(test.Metrics.TotalRequests)*100)
				} else {
					md += fmt.Sprintf("| Success Rate | 0.00%% |\n")
				}
				md += fmt.Sprintf("| Throughput | %.2f req/sec |\n", test.Metrics.RequestsPerSecond)
				md += fmt.Sprintf("| Avg Latency | %.2f ms |\n", test.Metrics.AvgLatencyMs)
				md += fmt.Sprintf("| P50 Latency | %d ms |\n", test.Metrics.P50LatencyMs)
				md += fmt.Sprintf("| P95 Latency | %d ms |\n", test.Metrics.P95LatencyMs)
				md += fmt.Sprintf("| P99 Latency | %d ms |\n", test.Metrics.P99LatencyMs)
				md += fmt.Sprintf("| Min Latency | %d ms |\n", test.Metrics.MinLatencyMs)
				md += fmt.Sprintf("| Max Latency | %d ms |\n\n", test.Metrics.MaxLatencyMs)
			}

			if len(test.Observations) > 0 {
				md += "**Observations:**\n\n"
				for _, obs := range test.Observations {
					md += fmt.Sprintf("- %s\n", obs)
				}
				md += "\n"
			}
		}
	}

	// Conclusions
	md += "## Conclusions\n\n"
	for _, conclusion := range r.Conclusions {
		md += fmt.Sprintf("- %s\n", conclusion)
	}
	md += "\n"

	// Recommendations
	md += "## Recommendations\n\n"
	for _, rec := range r.Recommendations {
		md += fmt.Sprintf("%s\n", rec)
	}
	md += "\n"

	// Appendix
	md += "## Appendix\n\n"
	md += "### Performance Grading Criteria\n\n"
	md += "- **A**: Excellent (Success Rate >99.9%, P95 <50ms, High throughput)\n"
	md += "- **B**: Good (Success Rate >99%, P95 <100ms, Adequate throughput)\n"
	md += "- **C**: Acceptable (Success Rate >95%, P95 <200ms)\n"
	md += "- **D**: Poor (Success Rate >90%, P95 <500ms)\n"
	md += "- **F**: Failure (Success Rate <90% or P95 >500ms)\n\n"

	md += "### Latency SLA Targets\n\n"
	md += "- **P50**: <20ms (target for 50% of requests)\n"
	md += "- **P95**: <100ms (target for 95% of requests)\n"
	md += "- **P99**: <200ms (target for 99% of requests)\n"
	md += "- **Max**: <500ms (absolute maximum acceptable latency)\n\n"

	md += fmt.Sprintf("---\n\n*Report generated at %s*\n",
		time.Now().Format("2006-01-02 15:04:05 MST"))

	return md
}

// GradePerformance assigns a performance grade based on metrics
func GradePerformance(metrics *PerformanceMetrics) string {
	if metrics == nil {
		return "N/A"
	}

	successRate := float64(metrics.SuccessfulReqs) / float64(metrics.TotalRequests) * 100

	// Grade A: Excellent
	if successRate >= 99.9 && metrics.P95LatencyMs <= 50 && metrics.RequestsPerSecond >= 500 {
		return "A"
	}

	// Grade B: Good
	if successRate >= 99.0 && metrics.P95LatencyMs <= 100 {
		return "B"
	}

	// Grade C: Acceptable
	if successRate >= 95.0 && metrics.P95LatencyMs <= 200 {
		return "C"
	}

	// Grade D: Poor
	if successRate >= 90.0 && metrics.P95LatencyMs <= 500 {
		return "D"
	}

	// Grade F: Failure
	return "F"
}

// GenerateObservations creates observations based on metrics
func GenerateObservations(metrics *PerformanceMetrics, config TestConfiguration) []string {
	if metrics == nil {
		return []string{}
	}

	observations := make([]string, 0)

	// Success rate observations
	successRate := float64(metrics.SuccessfulReqs) / float64(metrics.TotalRequests) * 100
	if successRate < 100 {
		observations = append(observations,
			fmt.Sprintf("%.2f%% success rate with %d failures", successRate, metrics.FailedReqs))
	}

	// Latency observations
	if metrics.P99LatencyMs > metrics.P95LatencyMs*2 {
		observations = append(observations,
			"Significant latency variance detected (P99 > 2x P95)")
	}

	if metrics.MaxLatencyMs > metrics.P99LatencyMs*3 {
		observations = append(observations,
			fmt.Sprintf("Outlier detected: max latency (%dms) significantly higher than P99", metrics.MaxLatencyMs))
	}

	// Throughput observations
	if config.TargetRPS > 0 {
		achievedPercent := (metrics.RequestsPerSecond / config.TargetRPS) * 100
		if achievedPercent < 90 {
			observations = append(observations,
				fmt.Sprintf("Only achieved %.1f%% of target throughput", achievedPercent))
		}
	}

	// Concurrency scaling observations
	if config.Concurrency > 50 && metrics.P95LatencyMs > 200 {
		observations = append(observations,
			"High concurrency may be causing resource contention")
	}

	return observations
}
