package main

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/m-lab/go/prometheusx/promtest"
	"github.com/urfave/cli/v3"
)

func TestMetrics(t *testing.T) {
	receiverDuration.WithLabelValues("x")
	promtest.LintMetrics(t)
}

func Test_main(t *testing.T) {
	// Prevent cli.Exit from calling os.Exit during tests.
	origOsExiter := cli.OsExiter
	cli.OsExiter = func(int) {}
	t.Cleanup(func() { cli.OsExiter = origOsExiter })
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name: "okay-default",
			args: []string{
				"github_receiver",
				"--org", "fake-org",
				"--repo", "fake-repo",
				"--authtoken", "token",
				"--webhook.listen-address", ":0",
				"--prometheus.listen-address", ":0",
			},
		},
		{
			name: "okay-inmemory",
			args: []string{
				"github_receiver",
				"--org", "fake-org",
				"--repo", "fake-repo",
				"--authtoken", "fake-token",
				"--enable-inmemory",
				"--webhook.listen-address", ":0",
				"--prometheus.listen-address", ":0",
			},
		},
		{
			name:        "missing-flags-usage",
			args:        []string{"github_receiver"},
			expectError: true,
		},
		{
			name: "bad-title-tmpl",
			args: []string{
				"github_receiver",
				"--org", "fake-org",
				"--repo", "fake-repo",
				"--authtoken", "token",
				"--title-template", "{{ x }}",
				"--webhook.listen-address", ":0",
				"--prometheus.listen-address", ":0",
			},
			expectError: true,
		},
		{
			name: "bad-baseURL",
			args: []string{
				"github_receiver",
				"--org", "fake-org",
				"--repo", "fake-repo",
				"--authtoken", "token",
				"--enterprise.base-url", "invalidURLEscape%zz",
				"--webhook.listen-address", ":0",
				"--prometheus.listen-address", ":0",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())

			var runErr error
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				runErr = newApp().Run(ctx, tt.args)
			}()

			// Cancel context to stop the server for the success cases.
			// Error cases exit before reaching the blocking <-ctx.Done().
			cancel()
			wg.Wait()

			if tt.expectError {
				if runErr == nil {
					t.Error("expected an error but got nil")
				}
			} else if runErr != nil && runErr != context.Canceled {
				t.Errorf("unexpected error: %v", runErr)
			}
		})
	}
}

func Test_authtokenFile(t *testing.T) {
	// Write a temp file with a fake token.
	f, err := os.CreateTemp(t.TempDir(), "authtoken")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("fake-token"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	ctx, cancel := context.WithCancel(context.Background())
	args := []string{
		"github_receiver",
		"--org", "fake-org",
		"--repo", "fake-repo",
		"--authtoken-file", f.Name(),
		"--enable-inmemory",
		"--webhook.listen-address", ":0",
		"--prometheus.listen-address", ":0",
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	var runErr error
	go func() {
		defer wg.Done()
		runErr = newApp().Run(ctx, args)
	}()
	cancel()
	wg.Wait()

	if runErr != nil && runErr != context.Canceled {
		t.Errorf("unexpected error: %v", runErr)
	}
}
