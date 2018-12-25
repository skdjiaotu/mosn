/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package prometheus

import (
	"io/ioutil"
	"net/http"
	"testing"
	"github.com/alipay/sofa-mosn/pkg/stats"
	"bytes"
	"sync"
)

type testAction int

const (
	countInc        testAction = iota
	countDec
	gaugeUpdate
	histogramUpdate
)

// test concurrently add statisic data
// should get the right data from prometheus
func TestPrometheusMetrics(t *testing.T) {
	stats.ResetAll()
	testCases := []struct {
		typ         string
		labels      map[string]string
		key         string
		action      testAction
		actionValue int64
	}{
		{"t1", map[string]string{"lbk1": "lbv1"}, "k1", countInc, 1},
		{"t1", map[string]string{"lbk1": "lbv2"}, "k1", countInc, 1},
		{"t1", map[string]string{"lbk1": "lbv1"}, "k1", countDec, 1},
		{"t1", map[string]string{"lbk1": "lbv1"}, "k2", countInc, 1},
		{"t1", map[string]string{"lbk1": "lbv1"}, "k3", gaugeUpdate, 1},
		{"t1", map[string]string{"lbk1": "lbv1"}, "k4", histogramUpdate, 1},
		{"t1", map[string]string{"lbk1": "lbv1"}, "k4", histogramUpdate, 2},
		{"t1", map[string]string{"lbk1": "lbv1"}, "k4", histogramUpdate, 3},
		{"t1", map[string]string{"lbk1": "lbv1"}, "k4", histogramUpdate, 4},
		{"t1", map[string]string{"lbk2": "lbv2"}, "k1", countInc, 1},
		{"t1", map[string]string{"lbk2": "lbv2"}, "k2", countInc, 2},
		{"t1", map[string]string{"lbk2": "lbv2"}, "k3", gaugeUpdate, 3},
		{"t1", map[string]string{"lbk2": "lbv2"}, "k4", histogramUpdate, 2},
		{"t2", map[string]string{"lbk1": "lbv1"}, "k1", countInc, 1},
	}
	wg := sync.WaitGroup{}
	for i := range testCases {
		wg.Add(1)
		go func(i int) {
			tc := testCases[i]
			s, _ := stats.NewStats(tc.typ, tc.labels)
			switch tc.action {
			case countInc:
				s.Counter(tc.key).Inc(tc.actionValue)
			case countDec:
				s.Counter(tc.key).Dec(tc.actionValue)
			case gaugeUpdate:
				s.Gauge(tc.key).Update(tc.actionValue)
			case histogramUpdate:
				s.Histogram(tc.key).Update(tc.actionValue)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()

	sink := NewPromeSink(&promConfig{
		Port:                  8088,
		Endpoint:              "/metrics",
		DisableCollectProcess: true,
		DisableCollectGo:      true,
	})
	tc := http.Client{}
	sink.Flush(stats.GetAll())

	resp, _ := tc.Get("http://127.0.0.1:8088/metrics")
	body, _ := ioutil.ReadAll(resp.Body)

	if !bytes.Contains(body, []byte("t1_lbk1_k1{lbk1=\"lbv1\"} 0.0")) {
		t.Error("t1_lbk1_k1{lbk1=\"lbv1\"} metric not correct")
	}

	if !bytes.Contains(body, []byte("t1_lbk1_k1{lbk1=\"lbv2\"} 1.0")) {
		t.Error("t1_lbk1_k1{lbk1=\"lbv2\"} metric not correct")
	}

	if !bytes.Contains(body, []byte("t1_lbk1_k4{lbk1=\"lbv1\",type=\"max\"} 4.0")) {
		t.Error("t1_lbk1_k4{lbk1=\"lbv1\",type=\"max\"} metric not correct")
	}

	if !bytes.Contains(body, []byte("t1_lbk2_k4{lbk2=\"lbv2\",type=\"min\"} 2.0")) {
		t.Error("t1_lbk2_k4{lbk2=\"lbv2\",type=\"min\"} metric not correct")
	}
}
