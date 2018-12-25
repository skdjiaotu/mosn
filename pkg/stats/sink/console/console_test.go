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

package console

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"sync"
	"testing"

	"github.com/alipay/sofa-mosn/pkg/stats"
)

type testAction int

const (
	countInc        testAction = iota
	countDec
	gaugeUpdate
	histogramUpdate
)

// test concurrently add statisic data
// should get the right data from console
func TestConsoleMetrics(t *testing.T) {
	stats.ResetAll()
	testCases := []struct {
		typ         string
		labels   map[string]string
		key         string
		action      testAction
		actionValue int64
	}{
		{"t1", map[string]string{"lbk1": "lbv1"}, "k1", countInc, 1},
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

	buf := &bytes.Buffer{}
	NewConsoleSink(buf).Flush(stats.GetAll())
	datas := make(map[string]map[string]map[string]string)
	json.Unmarshal(buf.Bytes(), &datas)
	t1Data := datas["t1"]
	if ns1, ok := t1Data["lbk1.lbv1"]; !ok {
		t.Error("no lbk1.lbv1 data")
	} else {
		if !(ns1["k1"] == "0" &&
			ns1["k2"] == "1" &&
			ns1["k3"] == "1") {
			t.Error("count and gauge not expected")
		}
		//TODO: histogram value expected
	}
	if ns2, ok := t1Data["lbk2.lbv2"]; !ok {
		t.Error("no lbk2.lbv2 data")
	} else {
		if !(ns2["k1"] == "1" &&
			ns2["k2"] == "2" &&
			ns2["k3"] == "3") {
			t.Error("count and gauge not expected")
		}
		//TODO: histogram value expected
	}
	t2Data := datas["t2"]
	if ns1, ok := t2Data["lbk1.lbv1"]; !ok {
		t.Error("no lbk1.lbv1 data")
	} else {
		if ns1["k1"] != "1" {
			t.Error("k1 value not expected")
		}
	}
}

func BenchmarkGetMetrics(b *testing.B) {
	stats.ResetAll()
	// init metrics data
	testCases := []struct {
		typ    string
		labels map[string]string
	}{
		{stats.DownstreamType, map[string]string{"proxy": "global"}},
		{stats.DownstreamType, map[string]string{"listener": "1"}},
		{stats.DownstreamType, map[string]string{"listener": "2"}},
		{stats.UpstreamType, map[string]string{"cluster": "1"}},
		{stats.UpstreamType, map[string]string{"cluster": "2"}},
		{stats.UpstreamType, map[string]string{"cluster": "1", "host":"1"}},
		{stats.UpstreamType, map[string]string{"cluster": "1", "host":"2"}},
		{stats.UpstreamType, map[string]string{"cluster": "2", "host":"1"}},
		{stats.UpstreamType, map[string]string{"cluster": "2", "host":"2"}},
	}
	for _, tc := range testCases {
		s, _ := stats.NewStats(tc.typ, tc.labels)
		s.Counter("key1").Inc(100)
		s.Counter("key2").Inc(100)
		s.Gauge("key3").Update(100)
		for i := 0; i < 5; i++ {
			s.Histogram("key4").Update(1)
		}
	}
	sink := NewConsoleSink(ioutil.Discard)
	for i := 0; i < b.N; i++ {
		sink.Flush(stats.GetAll())
	}
}
