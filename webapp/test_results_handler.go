// Copyright 2017 The WPT Dashboard Project. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package webapp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/web-platform-tests/wpt.fyi/shared"
)

type testRunUIFilter struct {
	Products string
	Labels   string
	SHA      string
	Aligned  bool
	MaxCount *int
	Diff     bool
	TestRuns string
}

// This handler is responsible for all pages that display test results.
// It fetches the latest TestRun for each browser then renders the HTML
// page with the TestRuns encoded as JSON. The Polymer app picks those up
// and loads the summary files based on each entity's TestRun.ResultsURL.
//
// The browsers initially displayed to the user are defined in browsers.json.
// The JSON property "initially_loaded" is what controls this.
func testResultsHandler(w http.ResponseWriter, r *http.Request) {
	// Redirect legacy paths.
	path := mux.Vars(r)["path"]
	var redir string
	if path == "results" {
		redir = "/results/"
	} else if strings.Index(r.URL.Path, "/results/") != 0 {
		redir = fmt.Sprintf("/results/%s", path)
	}
	if redir != "" {
		params := ""
		if r.URL.RawQuery != "" {
			params = "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, redir+params, http.StatusTemporaryRedirect)
		return
	}

	filter, err := parseTestRunUIFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data := struct {
		// TestRuns are inlined marshalled JSON for arbitrary test runs (e.g. when
		// diffing), for runs which aren't fetchable via a URL or the api.
		Filter testRunUIFilter
		Query  string
	}{
		Filter: filter,
		Query:  r.URL.Query().Get("q"),
	}

	if err := templates.ExecuteTemplate(w, "results.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// parseTestRunUIFilter parses the standard TestRunFilter, as well as the extra
// diff params (diff, before, after).
func parseTestRunUIFilter(r *http.Request) (filter testRunUIFilter, err error) {
	testRunFilter, err := shared.ParseTestRunFilterParams(r)
	if err != nil {
		return filter, err
	}
	testRunFilter = testRunFilter.OrDefault()

	if testRunFilter.Labels != nil {
		data, _ := json.Marshal(testRunFilter.Labels.ToSlice())
		filter.Labels = string(data)
	}
	if !shared.IsLatest(testRunFilter.SHA) {
		filter.SHA = testRunFilter.SHA
	}
	if !testRunFilter.IsDefaultProducts() {
		data, _ := json.Marshal(testRunFilter.Products.Strings())
		filter.Products = string(data)
	}
	filter.MaxCount = testRunFilter.MaxCount
	filter.Aligned = testRunFilter.Aligned != nil && *testRunFilter.Aligned

	diff, err := shared.ParseBooleanParam(r, "diff")
	if err != nil {
		return filter, err
	}
	filter.Diff = diff != nil && *diff

	var beforeAndAfter shared.ProductSpecs
	if beforeAndAfter, err = shared.ParseBeforeAndAfterParams(r); err != nil {
		return filter, err
	} else if len(beforeAndAfter) > 0 {
		var bytes []byte
		if bytes, err = json.Marshal(beforeAndAfter.Strings()); err != nil {
			return filter, err
		}
		filter.Products = string(bytes)
		filter.Diff = true
	}
	return filter, nil
}

func unpackTestRun(base64Run string) (*shared.TestRun, error) {
	decoded, err := base64.URLEncoding.DecodeString(base64Run)
	if err != nil {
		return nil, err
	}
	var run shared.TestRun
	if err := json.Unmarshal([]byte(decoded), &run); err != nil {
		return nil, err
	}
	return &run, nil
}
