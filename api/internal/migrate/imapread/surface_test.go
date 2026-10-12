package imapread

import (
	"go/ast"
	"go/types"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

const (
	thisPkg      = "github.com/productdevbook/yuva/api/internal/migrate/imapread"
	clientPkg    = "github.com/emersion/go-imap/v2/imapclient"
	imapModule   = "github.com/emersion/go-imap/v2"
	imapTypesPkg = "github.com/emersion/go-imap/v2"
)

var allowedClientMethods = []string{
	"Capability", "Caps", "WaitGreeting", "Login", "Authenticate", "List", "Status", "Select", "Unselect",
	"UIDSearch", "Search", "Fetch", "Noop", "Logout", "Close",
}

var allowedClientFuncs = []string{"New"}

func TestOnlyImapreadImportsClient(t *testing.T) {
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedImports, Tests: true}
	pkgs, err := packages.Load(cfg, "github.com/productdevbook/yuva/api/...")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) < 10 {
		t.Fatalf("loaded only %d packages", len(pkgs))
	}
	for _, p := range pkgs {
		if p.PkgPath == thisPkg || p.PkgPath == thisPkg+"_test" {
			continue
		}
		for path := range p.Imports {
			if strings.HasPrefix(path, imapModule) || strings.HasPrefix(path, "github.com/emersion/go-sasl") {
				t.Errorf("%s imports %s; only %s may", p.ID, path, thisPkg)
			}
		}
	}
}

func TestImapreadCallsOnlyReads(t *testing.T) {
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax | packages.NeedFiles}
	pkgs, err := packages.Load(cfg, thisPkg)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 || len(pkgs[0].Errors) > 0 {
		t.Fatalf("load: %v", pkgs)
	}
	p := pkgs[0]
	selects, peeks := 0, 0
	for _, f := range p.Syntax {
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				obj := p.TypesInfo.Uses[n.Sel]
				if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != clientPkg {
					return true
				}
				fn, ok := obj.(*types.Func)
				if !ok {
					return true
				}
				sig := fn.Signature()
				if recv := sig.Recv(); recv != nil {
					if named := namedOf(recv.Type()); named == "Client" && !slices.Contains(allowedClientMethods, fn.Name()) {
						t.Errorf("%s: imapread calls (*imapclient.Client).%s", p.Fset.Position(n.Pos()), fn.Name())
					}
				} else if !slices.Contains(allowedClientFuncs, fn.Name()) {
					t.Errorf("%s: imapread calls imapclient.%s", p.Fset.Position(n.Pos()), fn.Name())
				}
			case *ast.CallExpr:
				sel, ok := n.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Select" {
					return true
				}
				fn, ok := p.TypesInfo.Uses[sel.Sel].(*types.Func)
				if !ok || fn.Pkg() == nil || fn.Pkg().Path() != clientPkg {
					return true
				}
				selects++
				if len(n.Args) != 2 || !literalTrue(n.Args[1], "ReadOnly") {
					t.Errorf("%s: Select without a literal ReadOnly: true", p.Fset.Position(n.Pos()))
				}
			case *ast.CompositeLit:
				tv, ok := p.TypesInfo.Types[n]
				if !ok {
					return true
				}
				switch typeName(tv.Type) {
				case imapTypesPkg + ".FetchItemBodySection":
					peeks++
					if !hasTrue(n, "Peek") {
						t.Errorf("%s: FetchItemBodySection without a literal Peek: true", p.Fset.Position(n.Pos()))
					}
				case imapTypesPkg + ".FetchItemBinarySection", imapTypesPkg + ".StoreFlags", imapTypesPkg + ".AppendOptions":
					t.Errorf("%s: imapread builds %s", p.Fset.Position(n.Pos()), typeName(tv.Type))
				}
			}
			return true
		})
	}
	if selects == 0 || peeks == 0 {
		t.Fatalf("found %d Select calls and %d body sections; the scan is not seeing the code", selects, peeks)
	}
}

func TestImapreadHidesClient(t *testing.T) {
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedTypes}
	pkgs, err := packages.Load(cfg, thisPkg)
	if err != nil || len(pkgs) != 1 {
		t.Fatal(err)
	}
	scope := pkgs[0].Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		check := func(where string, typ types.Type) {
			if strings.Contains(types.TypeString(typ, nil), imapModule) {
				t.Errorf("%s exposes %s", where, types.TypeString(typ, nil))
			}
		}
		check(name, obj.Type())
		if tn, ok := obj.(*types.TypeName); ok {
			if st, ok := tn.Type().Underlying().(*types.Struct); ok {
				for i := range st.NumFields() {
					if f := st.Field(i); f.Exported() {
						check(name+"."+f.Name(), f.Type())
					}
				}
			}
			ms := types.NewMethodSet(types.NewPointer(tn.Type()))
			for i := range ms.Len() {
				if m := ms.At(i).Obj(); m.Exported() {
					check(name+"."+m.Name(), m.Type())
				}
			}
		}
	}
}

func namedOf(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := t.(*types.Named); ok {
		return n.Obj().Name()
	}
	return ""
}

func typeName(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := t.(*types.Named); ok && n.Obj().Pkg() != nil {
		return n.Obj().Pkg().Path() + "." + n.Obj().Name()
	}
	return ""
}

func literalTrue(e ast.Expr, field string) bool {
	u, ok := e.(*ast.UnaryExpr)
	if !ok {
		return false
	}
	lit, ok := u.X.(*ast.CompositeLit)
	return ok && hasTrue(lit, field)
}

func hasTrue(lit *ast.CompositeLit, field string) bool {
	for _, el := range lit.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if k, ok := kv.Key.(*ast.Ident); ok && k.Name == field {
			v, ok := kv.Value.(*ast.Ident)
			return ok && v.Name == "true"
		}
	}
	return false
}
