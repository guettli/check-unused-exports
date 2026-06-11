// check-unused-exports reports exported identifiers (constants, variables,
// types, functions, methods) that are never referenced outside test files.
//
// Usage:
//
//	go run ./hack/tools/check-unused-exports [root-dir]
//
// Exits 0 when nothing is found, 1 when unused exports are found.
//
// Limitation: uses name-based matching. Two identifiers with the same name
// (e.g. a method and a function both called "New") are treated as one; a
// reference to either marks both as used.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type kind string

const (
	kindConst  kind = "constant"
	kindVar    kind = "variable"
	kindType   kind = "type"
	kindFunc   kind = "function"
	kindMethod kind = "method"
)

type declInfo struct {
	name string
	kind kind
	pos  token.Position
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [root-dir]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Reports exported identifiers not referenced outside test files.\n")
		fmt.Fprintf(os.Stderr, "Skips vendor/ and hidden directories.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}

	fset := token.NewFileSet()
	mainFiles, testFiles, err := parseFiles(fset, root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	decls, declPos := collectExportedDecls(fset, mainFiles)
	usedInMain := collectUsages(mainFiles, decls, declPos)
	usedInTest := collectUsages(testFiles, decls, declPos)

	var unused []declInfo
	for name, info := range decls {
		if !usedInMain[name] {
			unused = append(unused, info)
		}
	}

	sort.Slice(unused, func(i, j int) bool {
		pi, pj := unused[i].pos, unused[j].pos
		if pi.Filename != pj.Filename {
			return pi.Filename < pj.Filename
		}
		return pi.Line < pj.Line
	})

	for _, d := range unused {
		suffix := ""
		if usedInTest[d.name] {
			suffix = " (used in tests)"
		}
		fmt.Printf("%s: exported %s %s is not used%s\n", d.pos, d.kind, d.name, suffix)
	}

	if len(unused) > 0 {
		os.Exit(1)
	}
}

func parseFiles(fset *token.FileSet, root string) (mainFiles, testFiles []*ast.File, err error) {
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			// Don't filter the root itself, only subdirectories.
			if path != root {
				name := d.Name()
				if name == "vendor" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return fmt.Errorf("parsing %s: %w", path, parseErr)
		}
		if strings.HasSuffix(path, "_test.go") {
			testFiles = append(testFiles, f)
		} else {
			mainFiles = append(mainFiles, f)
		}
		return nil
	})
	return
}

// collectExportedDecls gathers every exported identifier declared at package
// level in the given files, together with a set of positions that are
// declaration sites (excluded from usage counting).
func collectExportedDecls(fset *token.FileSet, files []*ast.File) (map[string]declInfo, map[token.Pos]bool) {
	decls := make(map[string]declInfo)
	declPos := make(map[token.Pos]bool)

	add := func(ident *ast.Ident, k kind) {
		declPos[ident.Pos()] = true
		if ast.IsExported(ident.Name) {
			decls[ident.Name] = declInfo{name: ident.Name, kind: k, pos: fset.Position(ident.Pos())}
		}
	}

	for _, file := range files {
		for _, node := range file.Decls {
			switch d := node.(type) {
			case *ast.GenDecl:
				var k kind
				switch d.Tok {
				case token.CONST:
					k = kindConst
				case token.VAR:
					k = kindVar
				case token.TYPE:
					k = kindType
				default:
					continue
				}
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.ValueSpec: // const or var
						for _, name := range s.Names {
							add(name, k)
						}
					case *ast.TypeSpec: // type
						add(s.Name, k)
					}
				}
			case *ast.FuncDecl:
				if d.Name == nil {
					continue
				}
				k := kindFunc
				if d.Recv != nil && len(d.Recv.List) > 0 {
					k = kindMethod
				}
				add(d.Name, k)
			}
		}
	}
	return decls, declPos
}

// collectUsages walks the given files and records which declared names appear
// as identifiers outside their own declaration position.
func collectUsages(files []*ast.File, decls map[string]declInfo, declPos map[token.Pos]bool) map[string]bool {
	used := make(map[string]bool)
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			ident, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			if _, tracked := decls[ident.Name]; tracked && !declPos[ident.Pos()] {
				used[ident.Name] = true
			}
			return true
		})
	}
	return used
}
