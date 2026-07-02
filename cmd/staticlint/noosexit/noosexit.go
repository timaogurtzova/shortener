// Package noosexit запрещает прямой вызов os.Exit в функции main пакета main.
package noosexit

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer проверяет, что функция main пакета main не вызывает os.Exit напрямую.
var Analyzer = &analysis.Analyzer{
	Name: "noosexit",
	Doc:  "noosexit запрещает прямой вызов os.Exit в функции main пакета main",
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
	Run: run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	if pass.Pkg.Name() != "main" || strings.HasSuffix(pass.Pkg.Path(), ".test") {
		return nil, nil
	}

	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, nil
	}

	inspect.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(node ast.Node) {
		fn, ok := node.(*ast.FuncDecl)
		if !ok {
			return
		}

		if fn.Recv != nil || fn.Name.Name != "main" || fn.Body == nil {
			return
		}

		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			if isOSExitCall(pass, call) {
				pass.Reportf(call.Pos(), "direct os.Exit call in main function is forbidden")
			}

			return true
		})
	})

	return nil, nil
}

func isOSExitCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Exit" {
		return false
	}

	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}

	pkgName, ok := pass.TypesInfo.Uses[ident].(*types.PkgName)
	return ok && pkgName.Imported().Path() == "os"
}
