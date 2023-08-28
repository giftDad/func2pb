package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/urfave/cli"
)

// GenFile 文件信息
type GenFile struct {
	Funcs   []GenFunc
	Structs []GenStruct
	Sn      string
}

// GenFunc 函数信息
type GenFunc struct {
	Name    string
	Comment string
	In      []GenField
	Out     []GenField
}

// GenStruct 结构体信息
type GenStruct struct {
	Name    string
	Comment string
	Field   []GenField
}

// GenField 字段信息
type GenField struct {
	Name    string
	Typ     string
	Comment string
}

func gen(c *cli.Context) error {
	// 获取文件Ast解析信息
	f, e := getAST(c)
	if e != nil {
		return e
	}

	// 生成PB
	r, err := genPB(f)
	if err != nil {
		return err
	}

	// 输出
	return doOut(c.String("out"), r)
}

func doOut(path string, content []byte) error {
	// out 未指定输出到stdout
	if path == "" {
		fmt.Println(string(content))
		return nil
	}

	// 创建文件
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close() // 确保在函数结束时关闭文件

	// 写入内容
	_, err = file.Write(content)
	return err
}

func getAST(c *cli.Context) (f GenFile, err error) {
	file := c.String("file")
	if file == "" {
		file, _ = os.Getwd()
	}
	// 服务名
	f.Sn = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	f.Sn = strings.ToUpper(string(f.Sn[0])) + strings.ToLower(f.Sn[1:])

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return f, err
	}

	fset := token.NewFileSet()
	fast, err := parser.ParseFile(fset, file, string(data), parser.ParseComments)
	if err != nil {
		return f, err
	}

	fuc := c.String("function")
	for _, decl := range fast.Decls {
		if fn, isFunc := decl.(*ast.FuncDecl); isFunc {
			if !ast.IsExported(fn.Name.Name) {
				continue
			}

			if fuc != "" && fn.Name.Name != fuc {
				continue
			}

			t := GenFunc{Name: fn.Name.Name, Comment: fn.Doc.Text()}

			for _, param := range fn.Type.Params.List {
				addPackageStruct(param.Type, fast)
				for _, n := range param.Names {
					if n.Name == "ctx" {
						continue
					}

					ppp := GenField{Name: n.Name, Typ: getTypeName(param.Type)}
					t.In = append(t.In, ppp)
				}
			}

			for index, param := range fn.Type.Results.List {
				addPackageStruct(param.Type, fast)
				ty := getTypeName(param.Type)
				if ty == "error" {
					continue
				}
				// 处理匿名返回值
				if len(param.Names) == 0 {
					ppp := GenField{Name: "res" + strconv.Itoa(index), Typ: ty}
					t.Out = append(t.Out, ppp)
				} else {
					for _, n := range param.Names {
						ppp := GenField{Name: n.Name, Typ: ty}
						t.Out = append(t.Out, ppp)
					}
				}

			}
			f.Funcs = append(f.Funcs, t)
		}

		for len(ps) != 0 {
		Exit:
			for _, decl := range fast.Decls {
				if genDecl, isGenDecl := decl.(*ast.GenDecl); isGenDecl {
					for _, spec := range genDecl.Specs {
						if typeSpec, isTypeSpec := spec.(*ast.TypeSpec); isTypeSpec {
							// 递归寻找struct
							if typeSpec.Name.String() != ps[0] {
								continue
							} else {
								ps = ps[1:]
								createdps[typeSpec.Name.String()] = true
							}

							s := GenStruct{Name: typeSpec.Name.String(), Comment: genDecl.Doc.Text()}
							if structType, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
								for _, field := range structType.Fields.List {
									addPackageStruct(field.Type, fast)
									for _, name := range field.Names {
										s.Field = append(s.Field, GenField{
											Name:    name.Name,
											Typ:     getTypeName(field.Type),
											Comment: field.Comment.Text(),
										})
									}
								}
							}
							f.Structs = append(f.Structs, s)
							break Exit
						}
					}
				}

			}

		}
	}
	return
}

// getTypeName 获取输出pb的name
func getTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.ArrayType:
		elemType := getTypeName(t.Elt)
		return "repeated " + elemType
	case *ast.MapType:
		keyType := getTypeName(t.Key)
		valueType := getTypeName(t.Value)
		return "map<" + keyType + ", " + valueType + ">"
	case *ast.StarExpr:
		starType := getTypeName(t.X)
		return starType
	case *ast.SelectorExpr:
		// 时间使用时间戳
		selectorType := getTypeName(t.X)
		if selectorType == "time" {
			return "int64"
		}
		return selectorType
	default:
		return "unknown"
	}
}

// isSame 是否是同一个结构体
func isSame(expr ast.Expr, name string) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == name
	case *ast.ArrayType:
		elemType := getTypeName(t.Elt)
		return elemType == name
	case *ast.MapType:
		keyType := getTypeName(t.Key)
		valueType := getTypeName(t.Value)
		return keyType == name || valueType == name
	case *ast.StarExpr:
		starType := getTypeName(t.X)
		return starType == name
	case *ast.SelectorExpr:
		// 时间使用时间戳
		selectorType := getTypeName(t.X)
		if selectorType == "time" {
			return false
		}
		return selectorType == name
	default:
		return false
	}
}

var ps []string
var createdps = make(map[string]bool)

// addPackageStruct 判断是否是包内结构体并加入队列
func addPackageStruct(expr ast.Expr, node *ast.File) {
	for _, decl := range node.Decls {
		if genDecl, isGenDecl := decl.(*ast.GenDecl); isGenDecl {
			for _, spec := range genDecl.Specs {
				if typeSpec, isTypeSpec := spec.(*ast.TypeSpec); isTypeSpec {
					if isSame(expr, typeSpec.Name.Name) && !createdps[typeSpec.Name.Name] {
						ps = append(ps, typeSpec.Name.Name)
					}
				}
			}
		}
	}
}

func genPB(f GenFile) (data []byte, err error) {
	buf := &bytes.Buffer{}
	tpl := template.New("rule").Funcs(template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
		"replace": func(s string) string {
			switch s {
			case "float32":
				return "float"
			case "float64":
				return "double"
			default:
				return s
			}
		},
		"rename": func(s string) string {
			var words []string
			// 找到所有在小写字母后面的大写字母
			var offset int
			for i := 1; i < len(s); i++ {
				if s[i] >= 'A' && s[i] <= 'Z' && s[i-1] >= 'a' && s[i-1] <= 'z' {
					words = append(words, strings.ToLower(s[offset:i]))
					offset = i
				}
			}
			words = append(words, strings.ToLower(s[offset:]))
			return strings.Join(words, "_")
		},
		"comment": func(input string) string {
			if input != "" {
				return "// " + strings.ReplaceAll(input, "\n", "") + "\n"
			}
			return input
		},
		"commentnotempty": func(input string) bool {
			input = strings.ReplaceAll(input, "\n", "")
			return input != ""
		},
	})

	template.Must(tpl.Parse(tmplPB))

	if err := tpl.Execute(buf, f); err != nil {
		panic(err)
	}

	return buf.Bytes(), nil
}

const tmplPB = `syntax = "proto3";

// TODO fill it
package xxx;

service {{ .Sn }} { {{ range .Funcs }}
{{ if commentnotempty .Comment }}	{{ comment .Comment }}{{ end }}	rpc {{ .Name }}({{ .Name }}Req) returns ({{ .Name }}Resp);{{ end }}
}

{{ range .Funcs }}
message {{ .Name }}Req { {{ range $index, $element := .In }}
	{{ replace .Typ }} {{ rename .Name }} = {{$index | add 1}};{{ end }}
}

message {{ .Name }}Resp {
	int32 code = 1;
	string msg = 2;
	{{ .Name }}Data data = 3;
}

message {{ .Name }}Data { {{ range $index, $element := .Out }}
	{{ replace .Typ }} {{ rename .Name }} = {{$index | add 1}};{{ end }}
}
{{ end }}

{{ range .Structs }}
{{ if commentnotempty .Comment }}{{ comment .Comment }}{{ end }}message {{ .Name }} { {{ range $index, $element := .Field }}
{{ if commentnotempty .Comment }}	{{ comment .Comment }}{{ end }}	{{ replace .Typ }} {{ rename .Name }} = {{$index | add 1}};{{ end }}
}
{{ end }}
`
