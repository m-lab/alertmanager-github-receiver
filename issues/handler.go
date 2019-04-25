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

package issues

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"

	"github.com/google/go-github/github"
)

const (
	listRawHTMLTemplate = `
<html><body>
<h2>Open Issues</h2>
<table>
{{range .}}
  <tr><td><a href={{.HTMLURL}}>{{.Title}}</a></td></tr>
{{end}}
</table>
<br/>
Receiver metrics: <a href="/metrics" onclick="javascript:event.target.port=9990">/metrics</a>
</body></html>`
)

var (
	listTemplate = template.Must(template.New("list").Parse(listRawHTMLTemplate))
)

// ListClient defines an interface for listing issues.
type ListClient interface {
	ListOpenIssues() ([]*github.Issue, error)
}

// ListHandler contains data needed for HTTP handlers.
type ListHandler struct {
	ListClient
}

// ServeHTTP lists open issues from github for view in a browser.
func (lh *ListHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" || req.Method != http.MethodGet {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(rw, "Wrong method\n")
		return
	}
	issues, err := lh.ListOpenIssues()
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(rw, "%s\n", err)
		return
	}
	var buf bytes.Buffer
	err = listTemplate.Execute(&buf, &issues)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(rw, "%s\n", err)
		return
	}
	rw.Write(buf.Bytes())
}
