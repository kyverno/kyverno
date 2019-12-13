// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package source

import (
	"context"
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/internal/lsp/protocol"
	"golang.org/x/tools/internal/telemetry/trace"
)

func DocumentSymbols(ctx context.Context, snapshot Snapshot, f File) ([]protocol.DocumentSymbol, error) {
	ctx, done := trace.StartSpan(ctx, "source.DocumentSymbols")
	defer done()

	pkg, pgh, err := getParsedFile(ctx, snapshot, f, NarrowestCheckPackageHandle)
	if err != nil {
		return nil, fmt.Errorf("getting file for DocumentSymbols: %v", err)
	}
	file, m, _, err := pgh.Cached()
	if err != nil {
		return nil, err
	}

	info := pkg.GetTypesInfo()
	q := qualifier(file, pkg.GetTypes(), info)

	methodsToReceiver := make(map[types.Type][]protocol.DocumentSymbol)
	symbolsToReceiver := make(map[types.Type]int)
	var symbols []protocol.DocumentSymbol
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if obj := info.ObjectOf(decl.Name); obj != nil {
				fs, err := funcSymbol(ctx, snapshot.View(), m, decl, obj, q)
				if err != nil {
					return nil, err
				}
				// Store methods separately, as we want them to appear as children
				// of the corresponding type (which we may not have seen yet).
				if fs.Kind == protocol.Method {
					rtype := obj.Type().(*types.Signature).Recv().Type()
					methodsToReceiver[rtype] = append(methodsToReceiver[rtype], fs)
				} else {
					symbols = append(symbols, fs)
				}
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					if obj := info.ObjectOf(spec.Name); obj != nil {
						ts, err := typeSymbol(ctx, snapshot.View(), m, info, spec, obj, q)
						if err != nil {
							return nil, err
						}
						symbols = append(symbols, ts)
						symbolsToReceiver[obj.Type()] = len(symbols) - 1
					}
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if obj := info.ObjectOf(name); obj != nil {
							vs, err := varSymbol(ctx, snapshot.View(), m, decl, name, obj, q)
							if err != nil {
								return nil, err
							}
							symbols = append(symbols, vs)
						}
					}
				}
			}
		}
	}

	// Attempt to associate methods to the corresponding type symbol.
	for typ, methods := range methodsToReceiver {
		if ptr, ok := typ.(*types.Pointer); ok {
			typ = ptr.Elem()
		}

		if i, ok := symbolsToReceiver[typ]; ok {
			symbols[i].Children = append(symbols[i].Children, methods...)
		} else {
			// The type definition for the receiver of these methods was not in the document.
			symbols = append(symbols, methods...)
		}
	}
	return symbols, nil
}

func funcSymbol(ctx context.Context, view View, m *protocol.ColumnMapper, decl *ast.FuncDecl, obj types.Object, q types.Qualifier) (protocol.DocumentSymbol, error) {
	s := protocol.DocumentSymbol{
		Name: obj.Name(),
		Kind: protocol.Function,
	}
	var err error
	s.Range, err = nodeToProtocolRange(ctx, view, m, decl)
	if err != nil {
		return protocol.DocumentSymbol{}, err
	}
	s.SelectionRange, err = nodeToProtocolRange(ctx, view, m, decl.Name)
	if err != nil {
		return protocol.DocumentSymbol{}, err
	}
	sig, _ := obj.Type().(*types.Signature)
	if sig != nil {
		if sig.Recv() != nil {
			s.Kind = protocol.Method
		}
		s.Detail += "("
		for i := 0; i < sig.Params().Len(); i++ {
			if i > 0 {
				s.Detail += ", "
			}
			param := sig.Params().At(i)
			label := types.TypeString(param.Type(), q)
			if param.Name() != "" {
				label = fmt.Sprintf("%s %s", param.Name(), label)
			}
			s.Detail += label
		}
		s.Detail += ")"
	}
	return s, nil
}

func setKind(s *protocol.DocumentSymbol, typ types.Type, q types.Qualifier) {
	switch typ := typ.Underlying().(type) {
	case *types.Interface:
		s.Kind = protocol.Interface
	case *types.Struct:
		s.Kind = protocol.Struct
	case *types.Signature:
		s.Kind = protocol.Function
		if typ.Recv() != nil {
			s.Kind = protocol.Method
		}
	case *types.Named:
		setKind(s, typ.Underlying(), q)
	case *types.Basic:
		i := typ.Info()
		switch {
		case i&types.IsNumeric != 0:
			s.Kind = protocol.Number
		case i&types.IsBoolean != 0:
			s.Kind = protocol.Boolean
		case i&types.IsString != 0:
			s.Kind = protocol.String
		}
	default:
		s.Kind = protocol.Variable
	}
}

func typeSymbol(ctx context.Context, view View, m *protocol.ColumnMapper, info *types.Info, spec *ast.TypeSpec, obj types.Object, q types.Qualifier) (protocol.DocumentSymbol, error) {
	s := protocol.DocumentSymbol{
		Name: obj.Name(),
	}
	s.Detail, _ = formatType(obj.Type(), q)
	setKind(&s, obj.Type(), q)

	var err error
	s.Range, err = nodeToProtocolRange(ctx, view, m, spec)
	if err != nil {
		return protocol.DocumentSymbol{}, err
	}
	s.SelectionRange, err = nodeToProtocolRange(ctx, view, m, spec.Name)
	if err != nil {
		return protocol.DocumentSymbol{}, err
	}
	t, objIsStruct := obj.Type().Underlying().(*types.Struct)
	st, specIsStruct := spec.Type.(*ast.StructType)
	if objIsStruct && specIsStruct {
		for i := 0; i < t.NumFields(); i++ {
			f := t.Field(i)
			child := protocol.DocumentSymbol{
				Name: f.Name(),
				Kind: protocol.Field,
			}
			child.Detail, _ = formatType(f.Type(), q)

			spanNode, selectionNode := nodesForStructField(i, st)
			if span, err := nodeToProtocolRange(ctx, view, m, spanNode); err == nil {
				child.Range = span
			}
			if span, err := nodeToProtocolRange(ctx, view, m, selectionNode); err == nil {
				child.SelectionRange = span
			}
			s.Children = append(s.Children, child)
		}
	}

	ti, objIsInterface := obj.Type().Underlying().(*types.Interface)
	ai, specIsInterface := spec.Type.(*ast.InterfaceType)
	if objIsInterface && specIsInterface {
		for i := 0; i < ti.NumExplicitMethods(); i++ {
			method := ti.ExplicitMethod(i)
			child := protocol.DocumentSymbol{
				Name: method.Name(),
				Kind: protocol.Method,
			}

			var spanNode, selectionNode ast.Node
		Methods:
			for _, f := range ai.Methods.List {
				for _, id := range f.Names {
					if id.Name == method.Name() {
						spanNode, selectionNode = f, id
						break Methods
					}
				}
			}
			child.Range, err = nodeToProtocolRange(ctx, view, m, spanNode)
			if err != nil {
				return protocol.DocumentSymbol{}, err
			}
			child.SelectionRange, err = nodeToProtocolRange(ctx, view, m, selectionNode)
			if err != nil {
				return protocol.DocumentSymbol{}, err
			}
			s.Children = append(s.Children, child)
		}

		for i := 0; i < ti.NumEmbeddeds(); i++ {
			embedded := ti.EmbeddedType(i)
			nt, isNamed := embedded.(*types.Named)
			if !isNamed {
				continue
			}

			child := protocol.DocumentSymbol{
				Name: types.TypeString(embedded, q),
			}
			setKind(&child, embedded, q)
			var spanNode, selectionNode ast.Node
		Embeddeds:
			for _, f := range ai.Methods.List {
				if len(f.Names) > 0 {
					continue
				}

				if t := info.TypeOf(f.Type); types.Identical(nt, t) {
					spanNode, selectionNode = f, f.Type
					break Embeddeds
				}
			}
			child.Range, err = nodeToProtocolRange(ctx, view, m, spanNode)
			if err != nil {
				return protocol.DocumentSymbol{}, err
			}
			child.SelectionRange, err = nodeToProtocolRange(ctx, view, m, selectionNode)
			if err != nil {
				return protocol.DocumentSymbol{}, err
			}
			s.Children = append(s.Children, child)
		}
	}
	return s, nil
}

func nodesForStructField(i int, st *ast.StructType) (span, selection ast.Node) {
	j := 0
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			if i == j {
				return field, field.Type
			}
			j++
			continue
		}
		for _, name := range field.Names {
			if i == j {
				return field, name
			}
			j++
		}
	}
	return nil, nil
}

func varSymbol(ctx context.Context, view View, m *protocol.ColumnMapper, decl ast.Node, name *ast.Ident, obj types.Object, q types.Qualifier) (protocol.DocumentSymbol, error) {
	s := protocol.DocumentSymbol{
		Name: obj.Name(),
		Kind: protocol.Variable,
	}
	if _, ok := obj.(*types.Const); ok {
		s.Kind = protocol.Constant
	}
	var err error
	s.Range, err = nodeToProtocolRange(ctx, view, m, decl)
	if err != nil {
		return protocol.DocumentSymbol{}, err
	}
	s.SelectionRange, err = nodeToProtocolRange(ctx, view, m, name)
	if err != nil {
		return protocol.DocumentSymbol{}, err
	}
	s.Detail = types.TypeString(obj.Type(), q)
	return s, nil
}
