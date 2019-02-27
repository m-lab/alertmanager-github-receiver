package issues

import (
	"testing"

	"github.com/m-lab/go/prometheusx"
)

func TestMetrics(t *testing.T) {
	rateLimit.WithLabelValues("x")
	rateRemaining.WithLabelValues("x")
	rateResetTime.WithLabelValues("x")
	operationCount.WithLabelValues("x")
	prometheusx.LintMetrics(t)
}
