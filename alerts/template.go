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
	"bytes"
	"fmt"
	"github.com/prometheus/alertmanager/notify"
	"html/template"
	"log"
)

const (
	// alertMD reports all alert labels and annotations in a markdown format
	// that renders correctly in github issues.
	//
	// Example:
	//
	// Alertmanager URL: http://localhost:9093
	//
	//  * firing
	//
	//	Labels:
	//
	//	 - alertname = DiskRunningFull
	//	 - dev = sda1
	//	 - instance = example1
	//
	//	Annotations:
	//
	//	 - test = value
	//
	//  * firing
	//
	//	Labels:
	//
	//	 - alertname = DiskRunningFull
	//	 - dev = sda2
	//   - instance = example2
	alertMD = `
Alertmanager URL: {{.Data.ExternalURL}}
{{range .Data.Alerts}}
  * {{.Status}} {{.GeneratorURL}}
  {{if .Labels}}
    Labels:
  {{- end}}
  {{range $key, $value := .Labels}}
    - {{$key}} = {{$value -}}
  {{end}}
  {{if .Annotations}}
    Annotations:
  {{- end}}
  {{range $key, $value := .Annotations}}
    - {{$key}} = {{$value -}}
  {{end}}
{{end}}

TODO: add graph url from annotations.
`
)

var (
	alertTemplate = template.Must(template.New("alert").Parse(alertMD))
)

func id(msg *notify.WebhookMessage) string {
	return fmt.Sprintf("0x%x", msg.GroupKey)
}

// formatTitle constructs an issue title from a webhook message.
func formatTitle(msg *notify.WebhookMessage) string {
	return fmt.Sprintf("%s", msg.Data.GroupLabels["alertname"])
}

// formatIssueBody constructs an issue body from a webhook message.
func formatIssueBody(msg *notify.WebhookMessage) string {
	var buf bytes.Buffer
	err := alertTemplate.Execute(&buf, msg)
	if err != nil {
		log.Printf("Error executing template: %s", err)
		return ""
	}
	s := buf.String()
	return fmt.Sprintf("<!-- ID: %s -->\n%s", id(msg), s)
}
