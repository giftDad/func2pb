package main

import (
	"bytes"
	"errors"
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

type FF struct {
	Funcs   []TT
	Structs []SS
	Sn      string
}

type TT struct {
	Name    string
	Comment string
	In      []p
	Out     []p
}

type SS struct {
	Name    string
	Comment string
	Field   []p
}

type p struct {
	Name    string
	Typ     string
	Comment string
}

func gen(c *cli.Context) error {
	out := c.String("out")
	if out == "" {
		return errors.New("output file is empty")
	}
	sn := strings.TrimSuffix(filepath.Base(out), filepath.Ext(out))
	sn = strings.ToUpper(string(sn[0])) + strings.ToLower(sn[1:])

	// 获取函数入参 出参
	fs, ss, e := getAST(c)
	if e != nil {
		return e
	}

	r, err := genPB(fs, ss, sn)
	if err != nil {
		return err
	}

	return doOut(out, r)
}

func doOut(path string, content []byte) error {
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

func getAST(c *cli.Context) (fs []TT, ss []SS, err error) {
	file := c.String("file")
	if file == "" {
		file, _ = os.Getwd()
	}

	fset := token.NewFileSet()

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return fs, ss, err
	}

	fast, err := parser.ParseFile(fset, file, string(data), parser.ParseComments)
	if err != nil {
		return fs, ss, err
	}

	for _, decl := range fast.Decls {
		if fn, isFunc := decl.(*ast.FuncDecl); isFunc {
			if !ast.IsExported(fn.Name.Name) {
				continue
			}

			t := TT{Name: fn.Name.Name, Comment: fn.Doc.Text()}

			for _, param := range fn.Type.Params.List {
				for _, n := range param.Names {
					if n.Name == "ctx" {
						continue
					}

					ppp := p{Name: n.Name, Typ: getTypeName(param.Type)}
					t.In = append(t.In, ppp)
				}
			}

			for index, param := range fn.Type.Results.List {
				ty := getTypeName(param.Type)
				if ty == "error" {
					continue
				}
				// 处理匿名返回值
				if len(param.Names) == 0 {
					ppp := p{Name: "res" + strconv.Itoa(index), Typ: ty}
					t.Out = append(t.Out, ppp)
				} else {
					for _, n := range param.Names {
						ppp := p{Name: n.Name, Typ: ty}
						t.Out = append(t.Out, ppp)
					}
				}

			}
			fs = append(fs, t)
		} else if genDecl, isGenDecl := decl.(*ast.GenDecl); isGenDecl {
			for _, spec := range genDecl.Specs {
				if typeSpec, isTypeSpec := spec.(*ast.TypeSpec); isTypeSpec {
					s := SS{Name: typeSpec.Name.String(), Comment: genDecl.Doc.Text()}
					if structType, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
						for _, field := range structType.Fields.List {
							for _, name := range field.Names {
								s.Field = append(s.Field, p{
									Name:    name.Name,
									Typ:     getTypeName(field.Type),
									Comment: field.Comment.Text(),
								})
							}
						}
					}
					ss = append(ss, s)
				}
			}
		}
	}

	return
}

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

func genPB(fs []TT, ss []SS, sn string) (data []byte, err error) {
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
		"trim": func(input string) string {
			return strings.ReplaceAll(input, "\n", "")

		},
	})

	template.Must(tpl.Parse(tmplPB))

	if err := tpl.Execute(buf, FF{fs, ss, sn}); err != nil {
		panic(err)
	}

	return buf.Bytes(), nil
}

const tmplPB = `syntax = "proto3";

// TODO fill it
package xxx;

service {{ .Sn }} {
	{{ range .Funcs }}// {{ trim .Comment }}
	rpc {{ .Name }}({{ .Name }}Req) returns ({{ .Name }}Resp);
	{{ end }}
}

{{ range .Funcs }}
message {{ .Name }}Req {
	{{ range $index, $element := .In }}{{ replace .Typ }} {{ rename .Name }} = {{$index | add 1}};
	{{ end }}
}

message {{ .Name }}Resp {
	int32 code = 1;
	string msg = 2;
	{{ .Name }}Data data = 3;
}

message {{ .Name }}Data {
	{{ range $index, $element := .Out }}{{ replace .Typ }} {{ rename .Name }} = {{$index | add 1}};
	{{ end }}
}
{{ end }}

{{ range .Structs }}
// {{ trim .Comment }}
message {{ .Name }} {
	{{ range $index, $element := .Field }}// {{ trim .Comment }}
	{{ replace .Typ }} {{ rename .Name }} = {{$index | add 1}};
	{{ end }}
}
{{ end }}
`
