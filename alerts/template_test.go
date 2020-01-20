// Copyright 2017 alertmanager-github-receiver Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//////////////////////////////////////////////////////////////////////////////

package alerts

import (
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/alertmanager/notify/webhook"
	amtmpl "github.com/prometheus/alertmanager/template"
)

func Test_formatIssueBody(t *testing.T) {
	wh := createWebhookMessage("FakeAlertName", "firing", "")
	brokenTemplate := `
{{range .NOT_REAL_FIELD}}
    * {{.Status}}
{{end}}
	`
	alertTemplate = template.Must(template.New("alert").Parse(brokenTemplate))
	got := formatIssueBody(wh)
	if got != "" {
		t.Errorf("formatIssueBody() = %q, want empty string", got)
	}
}

func TestFormatTitle(t *testing.T) {
	dir, err := ioutil.TempDir("", "github-receiver")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	aName := filepath.Join(dir, "a.tmpl")
	bName := filepath.Join(dir, "b.tmpl")
	if err := ioutil.WriteFile(aName, []byte(`{{ template "b" . }}`), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(bName, []byte(`{{ define "b" }}b is {{ .Status }}{{ end }}`), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	rh, err := NewReceiver(&fakeClient{}, "default", false, nil, []string{aName, bName})
	if err != nil {
		t.Fatal(err)
	}

	msg := webhook.Message{
		Data: &amtmpl.Data{Status: "firing"},
	}
	title, err := rh.formatTitle(&msg)
	if err != nil {
		t.Error(err)
	}

	if title != "b is firing" {
		t.Error(title)
	}
}
