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
	TrueTyp int
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

	// 生成s2pb
	if c.Bool("vv") {
		s, err := genS2PB(f)
		if err != nil {
			return err
		}
		fmt.Println(string(s))
	} else {
		// 输出
		return doOut(c.String("out"), r)

	}
	return nil
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

	st := c.String("struct")
	if st != "" {
		ps = append(ps, st)
		createdps[st] = true
	}

	fuc := c.String("function")
	for _, decl := range fast.Decls {
		if fn, isFunc := decl.(*ast.FuncDecl); isFunc {
			if !ast.IsExported(fn.Name.Name) {
				continue
			}

			if st != "" || (fuc != "" && fn.Name.Name != fuc) {
				continue
			}

			t := GenFunc{Name: fn.Name.Name, Comment: fn.Doc.Text()}

			if fn.Recv != nil {
				for _, field := range fn.Recv.List {
					ppp := GenField{Name: field.Names[0].Name, Typ: getTypeName(field.Type)}
					t.In = append(t.In, ppp)
				}
			}

			if fn.Type.Params != nil {
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
			}

			if fn.Type.Results != nil {
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
							}
							s := GenStruct{Name: typeSpec.Name.String(), Comment: genDecl.Doc.Text()}
							if structType, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
								for _, field := range structType.Fields.List {
									addPackageStruct(field.Type, fast)

									if len(field.Names) == 0 {
										s.Field = append(s.Field, GenField{
											Name:    getTypeName(field.Type),
											Typ:     getTypeName(field.Type),
											TrueTyp: getTypeEnum(field.Type),
											Comment: field.Comment.Text(),
										})
									} else {
										for _, name := range field.Names {
											if !ast.IsExported(name.Name) {
												continue
											}

											s.Field = append(s.Field, GenField{
												Name:    name.Name,
												Typ:     getTypeName(field.Type),
												TrueTyp: getTypeEnum(field.Type),
												Comment: field.Comment.Text(),
											})
										}

									}
								}
							}
							f.Structs = append(f.Structs, s)
							break Exit
						}
					}
				}
			}
			ps = ps[1:]
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
		n := selectorType + t.Sel.Name
		if n == "timeTime" {
			return "int64"
		}
		return n
	default:
		return "unknown"
	}
}

// getTypeEnum 获取特殊类型 time - 1 /包内struct - 2
func getTypeEnum(expr ast.Expr) int {
	switch t := expr.(type) {
	case *ast.Ident:
		// 假设首字母大写的类型为结构体
		if strings.ToUpper(t.Name[:1]) == t.Name[:1] {
			return 2
		}
		return 0
	case *ast.StarExpr:
		return getTypeEnum(t.X)
	case *ast.SelectorExpr:
		selectorType := getTypeName(t.X)
		if selectorType == "time" {
			return 1
		}
		return 0
	default:
		return 0
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
						createdps[typeSpec.Name.Name] = true
					}
				}
			}
		}
	}
}
func toCamelCase(s string) string {
	var bs []byte
	// 找到所有在小写字母后面的大写字母
	var offset int
	for i := 1; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' && s[i-1] >= 'a' && s[i-1] <= 'z' {
			w := []byte(s[offset:i])
			bs = append(bs, bytes.ToUpper([]byte{w[0]})...)
			bs = append(bs, bytes.ToLower(w[1:])...)
			offset = i
		}
	}
	bs = append(bs, bytes.ToUpper([]byte{s[offset]})...)
	bs = append(bs, bytes.ToLower([]byte(s[offset+1:]))...)
	return string(bs)
}

func genS2PB(f GenFile) (data []byte, err error) {
	buf := &bytes.Buffer{}
	tpl := template.New("rule").Funcs(template.FuncMap{
		"tolower": func(s string) string {
			return strings.ToLower(s)
		},
		"marconv": func(s string, t int) string {
			if t == 1 {
				return "s." + s + ".Unix()"
			} else if t == 2 {
				return "s2pb" + s + "(s." + s + ")"
			}
			return "s." + s
		},
		"unmarconv": func(s string, t int) string {
			if t == 1 {
				return "time.Unix(s." + toCamelCase(s) + ", 0)"
			} else if t == 2 {
				return "pb2s" + toCamelCase(s) + "(s." + toCamelCase(s) + ")"
			}
			return "s." + toCamelCase(s)
		},
		"toCamelCase": toCamelCase,
	})

	template.Must(tpl.Parse(tmplS2PB))

	if err := tpl.Execute(buf, f); err != nil {
		panic(err)
	}

	return buf.Bytes(), nil
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
			case "int":
				return "int64"
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
message {{ .Name }}Req {{ "{" }}{{ range $index, $element := .In }}
	{{ replace .Typ }} {{ rename .Name }} = {{$index | add 1}};{{ end }}
}

message {{ .Name }}Resp {
	int32 code = 1;
	string msg = 2;
	{{ .Name }}Data data = 3;
}

message {{ .Name }}Data {{ "{" }}{{ range $index, $element := .Out }}
	{{ replace .Typ }} {{ rename .Name }} = {{$index | add 1}};{{ end }}
}
{{ end }}

{{ range .Structs }}
{{ if commentnotempty .Comment }}{{ comment .Comment }}{{ end }}message {{ .Name }} { {{ range $index, $element := .Field }}
{{ if commentnotempty .Comment }}	{{ comment .Comment }}{{ end }}	{{ replace .Typ }} {{ rename .Name }} = {{$index | add 1}};{{ end }}
}
{{ end }}
`

const tmplS2PB = `
{{ range .Structs }}
func s2pb{{ .Name }}(s {{ tolower $.Sn }}.{{ .Name }}) *{{ .Name }} {
	return &{{ .Name }}{ {{ range .Field }}
		{{ toCamelCase .Name }} : {{ marconv .Name .TrueTyp }},{{ end }}
	}
}

func pb2s{{ .Name }}(s *{{ .Name }}) {{ tolower $.Sn }}.{{ .Name }}{
	return {{ tolower $.Sn }}. {{ .Name }}{ {{ range .Field }}
		{{ .Name }} : {{ unmarconv .Name .TrueTyp }},{{ end }}
	}
}
{{ end }}
`
