package main

import (
	"testing"

	"github.com/m-lab/go/prometheusx"
)

func TestMetrics(t *testing.T) {
	receiverDuration.WithLabelValues("x")
	prometheusx.LintMetrics(t)
}
