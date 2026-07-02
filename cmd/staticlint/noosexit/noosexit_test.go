package noosexit_test

import (
	"testing"

	"github.com/timaogurtzova/shortener/cmd/staticlint/noosexit"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), noosexit.Analyzer, "directexit", "alias", "notmain")
}
