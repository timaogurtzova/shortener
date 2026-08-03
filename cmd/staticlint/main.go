package main

import (
	"go/ast"

	"github.com/gostaticanalysis/forcetypeassert"
	"github.com/gostaticanalysis/nilerr"
	"github.com/timaogurtzova/shortener/cmd/staticlint/noosexit"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/appends"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/defers"
	"golang.org/x/tools/go/analysis/passes/directive"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/framepointer"
	"golang.org/x/tools/go/analysis/passes/hostport"
	"golang.org/x/tools/go/analysis/passes/httpmux"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/reflectvaluecompare"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sigchanyzer"
	"golang.org/x/tools/go/analysis/passes/slog"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stdversion"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/testinggoroutine"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/timeformat"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"golang.org/x/tools/go/analysis/passes/unusedwrite"
	"golang.org/x/tools/go/analysis/passes/waitgroup"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"
)

func main() {
	multichecker.Main(analyzers()...)
}

func analyzers() []*analysis.Analyzer {
	checks := []*analysis.Analyzer{
		appends.Analyzer,
		asmdecl.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		deepequalerrors.Analyzer,
		defers.Analyzer,
		directive.Analyzer,
		errorsas.Analyzer,
		framepointer.Analyzer,
		hostport.Analyzer,
		httpmux.Analyzer,
		httpresponse.Analyzer,
		ifaceassert.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		nilness.Analyzer,
		printf.Analyzer,
		reflectvaluecompare.Analyzer,
		shadow.Analyzer,
		shift.Analyzer,
		sigchanyzer.Analyzer,
		slog.Analyzer,
		sortslice.Analyzer,
		stdmethods.Analyzer,
		stdversion.Analyzer,
		stringintconv.Analyzer,
		structtag.Analyzer,
		testinggoroutine.Analyzer,
		tests.Analyzer,
		timeformat.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,
		unusedwrite.Analyzer,
		waitgroup.Analyzer,
	}

	checks = appendLintAnalyzers(checks, staticcheck.Analyzers, nil)
	checks = appendLintAnalyzers(checks, stylecheck.Analyzers, map[string]struct{}{
		"ST1005": {},
	})
	checks = append(checks,
		nilerr.Analyzer,
		skipGeneratedDiagnostics(forcetypeassert.Analyzer),
		noosexit.Analyzer,
	)

	return checks
}

// skipGeneratedDiagnostics suppresses diagnostics from generated Go files.
// Generated protobuf code contains unchecked type assertions by design and
// must be changed only by regenerating it.
func skipGeneratedDiagnostics(analyzer *analysis.Analyzer) *analysis.Analyzer {
	wrapped := *analyzer
	run := analyzer.Run

	wrapped.Run = func(pass *analysis.Pass) (any, error) {
		generated := generatedFileNames(pass)
		filteredPass := *pass
		filteredPass.Report = func(diagnostic analysis.Diagnostic) {
			filename := pass.Fset.PositionFor(diagnostic.Pos, false).Filename
			if _, ok := generated[filename]; ok {
				return
			}
			pass.Report(diagnostic)
		}

		return run(&filteredPass)
	}

	return &wrapped
}

func generatedFileNames(pass *analysis.Pass) map[string]struct{} {
	files := make(map[string]struct{})
	for _, file := range pass.Files {
		if !ast.IsGenerated(file) {
			continue
		}

		filename := pass.Fset.PositionFor(file.Pos(), false).Filename
		files[filename] = struct{}{}
	}

	return files
}

func appendLintAnalyzers(dst []*analysis.Analyzer, src []*lint.Analyzer, selected map[string]struct{}) []*analysis.Analyzer {
	for _, item := range src {
		if selected != nil {
			if _, ok := selected[item.Analyzer.Name]; !ok {
				continue
			}
			delete(selected, item.Analyzer.Name)
		}

		dst = append(dst, initializedLintAnalyzer(item))
	}

	for name := range selected {
		panic("staticlint: analyzer " + name + " was not found")
	}

	return dst
}

func initializedLintAnalyzer(item *lint.Analyzer) *analysis.Analyzer {
	if item.Analyzer.URL == "" {
		return lint.InitializeAnalyzer(item).Analyzer
	}

	return item.Analyzer
}
