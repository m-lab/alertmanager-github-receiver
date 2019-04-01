package main

import (
	"testing"

	"github.com/m-lab/go/prometheusx/promtest"
)

func TestMetrics(t *testing.T) {
	receiverDuration.WithLabelValues("x")
	promtest.LintMetrics(t)
}
