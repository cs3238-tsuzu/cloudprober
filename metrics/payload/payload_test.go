// Copyright 2017-2019 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package payload

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/google/cloudprober/metrics"
	configpb "github.com/google/cloudprober/metrics/payload/proto"
)

var (
	testPtype  = "external"
	testProbe  = "testprobe"
	testTarget = "test-target"
)

func parserForTest(t *testing.T, agg bool) *Parser {
	testConf := `
	  aggregate_in_cloudprober: %v
		dist_metric {
			key: "op_latency"
			value {
				explicit_buckets: "1,10,100"
			}
		}
 `
	var c configpb.OutputMetricsOptions
	if err := proto.UnmarshalText(fmt.Sprintf(testConf, agg), &c); err != nil {
		t.Error(err)
	}
	p, err := NewParser(&c, testPtype, testProbe, metrics.CUMULATIVE, nil)
	if err != nil {
		t.Error(err)
	}

	return p
}

// testData encapsulates the test data.
type testData struct {
	varA, varB float64
	lat        []float64
}

// testEM returns an EventMetrics struct corresponding to the provided testData.
func testEM(ts time.Time, td *testData) *metrics.EventMetrics {
	d := metrics.NewDistribution([]float64{1, 10, 100})
	for _, sample := range td.lat {
		d.AddSample(sample)
	}
	return metrics.NewEventMetrics(ts).
		AddMetric("op_latency", d).
		AddMetric("time_to_running", metrics.NewFloat(td.varA)).
		AddMetric("time_to_ssh", metrics.NewFloat(td.varB)).
		AddLabel("ptype", testPtype).
		AddLabel("probe", testProbe).
		AddLabel("dst", testTarget)
}

func testPayload(td *testData) string {
	var latencyStrs []string
	for _, f := range td.lat {
		latencyStrs = append(latencyStrs, fmt.Sprintf("%f", f))
	}
	payloadLines := []string{
		fmt.Sprintf("time_to_running %f", td.varA),
		fmt.Sprintf("time_to_ssh %f", td.varB),
		fmt.Sprintf("op_latency %s", strings.Join(latencyStrs, ",")),
	}
	return strings.Join(payloadLines, "\n")
}

func testPayloadMetrics(t *testing.T, em *metrics.EventMetrics, td, etd *testData) {
	t.Helper()

	expectedEM := testEM(em.Timestamp, etd)
	if em.String() != expectedEM.String() {
		t.Errorf("Output metrics not aggregated correctly:\nGot:      %s\nExpected: %s", em.String(), expectedEM.String())
	}
}

func TestAggreagateInCloudprober(t *testing.T) {
	p := parserForTest(t, true)

	// First payload
	td := &testData{10, 30, []float64{3.1, 4.0, 13}}
	em := p.PayloadMetrics(nil, testPayload(td), testTarget)

	testPayloadMetrics(t, em, td, td)

	// Send another payload, cloudprober should aggregate the metrics.
	oldtd := td
	td = &testData{
		varA: 8,
		varB: 45,
		lat:  []float64{6, 14.1, 2.1},
	}
	etd := &testData{
		varA: oldtd.varA + td.varA,
		varB: oldtd.varB + td.varB,
		lat:  append(oldtd.lat, td.lat...),
	}

	em = p.PayloadMetrics(em, testPayload(td), testTarget)
	testPayloadMetrics(t, em, td, etd)
}

func TestNoAggregation(t *testing.T) {
	p := parserForTest(t, false)

	// First payload
	td := &testData{10, 30, []float64{3.1, 4.0, 13}}
	em := p.PayloadMetrics(nil, testPayload(td), testTarget)
	testPayloadMetrics(t, em, td, td)

	// Send another payload, cloudprober should not aggregate the metrics.
	td = &testData{
		varA: 8,
		varB: 45,
		lat:  []float64{6, 14.1, 2.1},
	}
	em = p.PayloadMetrics(em, testPayload(td), testTarget)
	testPayloadMetrics(t, em, td, td)
}
