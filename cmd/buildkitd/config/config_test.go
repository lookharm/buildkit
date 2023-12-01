package config

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
	"unsafe"

	"github.com/pelletier/go-toml"
)

type D struct {
	Int int `toml:"int"`
}

type C struct {
	D D `toml:"d"`
}

type B struct {
	C1 C `toml:"c1" comment:"c1 is c1"`
	C2 C `toml:"c2" comment:"c2 is c2"`
}

type A struct {
	Debug bool   `toml:"debug" comment:"Debug is bool"`
	X1    int    `toml:"x1" comment:"x1 is string"`
	X2    string `toml:"x2" comment:"x2 is string"`
	// B is B
	B   B            `toml:"b" comment:"B is B"`
	Map map[string]B `toml:"map" comment:"d is d"`
}

func traverse(node reflect.Type) {
	for i := 0; i < node.NumField(); i++ {
		field := node.Field(i)
		fieldName := field.Name
		// tagToml := field.Tag.Get("toml")
		fmt.Println("struct", node.Name(), "field", fieldName)
		if field.Type.Kind() == reflect.Struct {
			traverse(field.Type)
		}
	}
}

func TestNestedStruct(t *testing.T) {
	root := reflect.TypeOf(A{})
	traverse(root)
}

func TestExtractValue(t *testing.T) {
	// toml.Marshal()
	s := A{
		X1: 0,
		X2: "A string",
		B: B{
			C1: C{
				D: D{
					Int: 1,
				},
			},
			C2: C{
				D: D{
					Int: 1,
				},
			},
		},
	}
	root := reflect.TypeOf(s)
	rootVal := reflect.ValueOf(s)
	for i := 0; i < root.NumField(); i++ {
		field := root.Field(i)
		fieldName := field.Name

		val := rootVal.Field(i)

		switch field.Type.Kind() {
		case reflect.Int:
			{
				fmt.Printf("%v %v = %v\n", fieldName, "int", val.Interface())
			}
		case reflect.Int32:
			{
				fmt.Printf("%v %v = %v\n", fieldName, "int32", val.Interface())
			}
			// default:
			// 	{
			// 		panic("this kind is not support")
			// 	}
		}
	}
}

func TestTOML(t *testing.T) {
	out, err := toml.Marshal(A{
		Debug: true,
		X1:    0,
		X2:    "A string",
		B: B{
			C1: C{
				D: D{
					Int: 1,
				},
			},
			C2: C{
				D: D{
					Int: 1,
				},
			},
		},
		Map: map[string]B{
			"d1": {
				C1: C{
					D: D{
						Int: 2,
					},
				},
				C2: C{
					D: D{
						Int: 2,
					},
				},
			},
			"d2": {
				C1: C{
					D: D{
						Int: 3,
					},
				},
				C2: C{
					D: D{
						Int: 3,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(out))
}

func TestGenerateDocs(t *testing.T) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "config.go", nil, parser.ParseComments)
	if err != nil {
		fmt.Println("Error parsing code:", err)
		return
	}

	configType := reflect.TypeOf(Config{})
	for i := 0; i < configType.NumField(); i++ {
		field := configType.Field(i)
		fieldName := field.Name
		tagToml := field.Tag.Get("toml")

		comment := findComment(node, fieldName)
		if comment != nil {
			fmt.Printf("#%s\n", comment.Text[2:])
			fmt.Printf("[%v]\n", tagToml)
			fmt.Printf("\n")
		}
	}
}

func findComment(node *ast.File, fieldName string) *ast.Comment {
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != "Config" {
				continue
			}
			if structType, ok := typeSpec.Type.(*ast.StructType); ok {
				for _, field := range structType.Fields.List {
					for _, name := range field.Names {
						if name.Name == fieldName {
							if len(field.Doc.List) > 0 {
								return field.Doc.List[0]
							}
						}
					}
				}
			}
		}
	}

	return nil
}

func getTestTree() (*toml.Tree, error) {
	config := Config{
		// Debug:        true,
		// Trace:        true,
		// Root:         "/root",
		// Entitlements: []string{"asdf", "asdf"},
	}
	bytes, err := toml.Marshal(config)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(bytes))
	tree, err := toml.LoadBytes(bytes)
	if err != nil {
		return nil, err
	}

	return tree, nil
}

func SetUnexportedField(field reflect.Value, value interface{}) {
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(value))
}

func traverseAndSet(node1 *toml.Tree, node2 Node) {
	for k, v := range node1.Values() {
		switch n1 := v.(type) {
		case *toml.Tree:
			svalue := reflect.ValueOf(n1)
			f := svalue.Elem().FieldByName("comment")
			SetUnexportedField(f, node2.Values[k].Comment)
			traverseAndSet(n1, node2.Values[k])
		default:
			svalue := reflect.ValueOf(n1)
			f := svalue.Elem().FieldByName("comment")
			SetUnexportedField(f, node2.Values[k].Comment)
		}
	}
}

func TestLoadTree(t *testing.T) {
	config := Config{
		Debug:        true,
		Trace:        true,
		Root:         "/root",
		Entitlements: []string{"asdf", "asdf"},
	}
	bytes, err := toml.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}

	tree, err := toml.LoadBytes(bytes)
	if err != nil {
		t.Fatal(err)
	}

	tree2 := buildTree2()

	traverseAndSet(tree, tree2)

	b, _ := tree.Marshal()
	fmt.Println(string(b))
}

type Node struct {
	Values  map[string]Node
	Comment string
}

func traverseAST(node Node, structType *ast.StructType, depth int) {
	for _, field := range structType.Fields.List {
		tag := ""
		comment := ""
		for _, name := range field.Names {
			if field.Doc != nil {
				for _, v := range field.Doc.List {
					comment += v.Text[2:] + "\n"
				}
				comment = comment[1 : len(comment)-1]
			}

			space := ""
			for i := 0; i < depth; i++ {
				space += "  "
			}
			if field.Tag != nil {
				tag = reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1]).Get("toml")
			} else {
				tag = name.Name
			}
		}
		node.Values[tag] = Node{
			Values:  make(map[string]Node),
			Comment: comment,
		}

		if st, ok := field.Type.(*ast.StructType); ok {
			traverseAST(node.Values[tag], st, depth+1)
		}
		if iden, ok := field.Type.(*ast.Ident); ok && iden.Obj != nil {
			if len(field.Names) > 0 {
				depth += 1
			}
			if typeSpec, ok := iden.Obj.Decl.(*ast.TypeSpec); ok {
				traverseAST(node.Values[tag], typeSpec.Type.(*ast.StructType), depth)
			}
			if len(field.Names) > 0 {
				depth -= 1
			}
		}
		if star, ok := field.Type.(*ast.StarExpr); ok {
			if iden, ok := star.X.(*ast.Ident); ok && iden.Obj != nil {
				if typeSpec, ok := iden.Obj.Decl.(*ast.TypeSpec); ok {
					traverseAST(node.Values[tag], typeSpec.Type.(*ast.StructType), depth+1)
				}
			}
		}
	}
}

func buildTree2() Node {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "config.go", nil, parser.ParseComments)
	if err != nil {
		fmt.Println("Error parsing code:", err)
		return Node{}
	}

	node2 := Node{
		Values: make(map[string]Node),
	}

	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != "Config" {
				continue
			}
			if structType, ok := typeSpec.Type.(*ast.StructType); ok {
				traverseAST(node2, structType, 0)
			}
		}
	}

	return node2
}
