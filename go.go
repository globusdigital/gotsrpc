package gotsrpc

import (
	"fmt"
	"strings"
)

func (v *Value) isHTTPResponseWriter() bool {
	return v.StructType != nil && v.StructType.Name == "ResponseWriter" && v.StructType.Package == "net/http"
}
func (v *Value) isHTTPRequest() bool {
	return v.IsPtr && v.StructType != nil && v.StructType.Name == "Request" && v.StructType.Package == "net/http"
}

func (v *Value) goType(aliases map[string]string, packageName string) (t string) {
	if v.IsPtr {
		t = "*"
	}
	switch true {
	case v.Array != nil:
		t += "[]" + v.Array.Value.goType(aliases, packageName)
	case len(v.GoScalarType) > 0:
		t += v.GoScalarType
	case v.StructType != nil:
		if packageName != v.StructType.Package {
			t += aliases[v.StructType.Package] + "."
		}
		t += v.StructType.Name

	}
	return

}

func (v *Value) emptyLiteral(aliases map[string]string) (e string) {
	e = ""
	if v.IsPtr {
		e += "&"
	}
	switch true {
	case len(v.GoScalarType) > 0:
		switch v.GoScalarType {
		case "string":
			e += "\"\""
		case "float":
			return "float(0.0)"
		case "float32":
			return "float32(0.0)"
		case "float64":
			return "float64(0.0)"
		case "int":
			return "int(0)"
		case "int32":
			return "int32(0)"
		case "int64":
			return "int64(0)"
		case "bool":
			return "false"
		}
	case v.Array != nil:
		e += "[]" + v.Array.Value.emptyLiteral(aliases) + "{}"
	case v.StructType != nil:
		alias := aliases[v.StructType.Package]
		if alias != "" {
			e += alias + "."
		}
		e += v.StructType.Name + "{}"

	}
	return
}

func lcfirst(str string) string {
	return strfirst(str, strings.ToLower)
}

func ucfirst(str string) string {
	return strfirst(str, strings.ToUpper)
}

func strfirst(str string, strfunc func(string) string) string {
	res := ""
	for i, char := range str {
		if i == 0 {
			res += strfunc(string(char))
		} else {
			res += string(char)
		}

	}
	return res

}

func renderServiceProxies(services []*Service, fullPackageName string, packageName string, g *code) error {
	aliases := map[string]string{
		"net/http":                 "http",
		"github.com/foomo/gotsrpc": "gotsrpc",
	}
	r := strings.NewReplacer(".", "_", "/", "_", "-", "_")
	extractImports := func(fields []*Field) {
		for _, f := range fields {
			if f.Value.StructType != nil {
				st := f.Value.StructType
				if st.Package != fullPackageName {
					alias, ok := aliases[st.Package]
					if !ok {
						packageParts := strings.Split(st.Package, "/")
						beautifulAlias := packageParts[len(packageParts)-1]
						uglyAlias := r.Replace(st.Package)
						alias = beautifulAlias
						for _, otherAlias := range aliases {
							if otherAlias == beautifulAlias {
								alias = uglyAlias
								break
							}
						}
						aliases[st.Package] = alias
					}

				}
			}
		}
	}
	for _, s := range services {
		for _, m := range s.Methods {
			extractImports(m.Args)
		}
	}

	imports := ""
	for packageName, alias := range aliases {
		imports += alias + " \"" + packageName + "\"\n"
	}

	g.l(`
        // this file was auto generated by gotsrpc https://github.com/foomo/gotsrpc
        package ` + packageName + `
        import (
			` + imports + `
        )
    `)
	for _, service := range services {
		proxyName := service.Name + "GoTSRPCProxy"
		g.l(`
        type ` + proxyName + ` struct {
	        EndPoint string
			allowOrigin []string
	        service  *` + service.Name + `
        }

        func New` + proxyName + `(service *` + service.Name + `, endpoint string, allowOrigin []string) *` + proxyName + ` {
	        return &` + proxyName + `{
		        EndPoint: endpoint,
				allowOrigin : allowOrigin,
		        service:  service,
	        }
        }
        
        // ServeHTTP exposes your service
        func (p *` + proxyName + `) ServeHTTP(w http.ResponseWriter, r *http.Request) {

			for _, origin := range p.allowOrigin {
				w.Header().Add("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Credentials", "true")
	        if r.Method != "POST" {
		        gotsrpc.ErrorMethodNotAllowed(w)
		        return
	        }
		`)
		needsArgs := false
		for _, method := range service.Methods {
			if len(method.Args) > 0 {
				needsArgs = true
				break
			}
		}
		if needsArgs {
			g.l(`var args []interface{}`)
		}
		g.l(`switch gotsrpc.GetCalledFunc(r, p.EndPoint) {`)

		// indenting into switch cases
		g.ind(4)

		for _, method := range service.Methods {
			// a case for each method
			g.l("case \"" + method.Name + "\":")
			g.ind(1)
			callArgs := []string{}
			isSessionRequest := false
			if len(method.Args) > 0 {

				args := []string{}

				skipArgI := 0

				for argI, arg := range method.Args {

					if argI == 0 && arg.Value.isHTTPResponseWriter() {
						continue
					}
					if argI == 1 && arg.Value.isHTTPRequest() {
						isSessionRequest = true
						continue
					}

					args = append(args, arg.Value.emptyLiteral(aliases))
					switch arg.Value.GoScalarType {
					case "int64":
						callArgs = append(callArgs, fmt.Sprint(arg.Value.GoScalarType+"(args[", skipArgI, "].(float64))"))
					default:
						// assert
						callArgs = append(callArgs, fmt.Sprint("args[", skipArgI, "].("+arg.Value.goType(aliases, fullPackageName)+")"))

					}

					skipArgI++
				}
				g.l("args = []interface{}{" + strings.Join(args, ", ") + "}")
				g.l("err := gotsrpc.LoadArgs(args, r)")
				g.l("if err != nil {")
				g.ind(1)
				g.l("gotsrpc.ErrorCouldNotLoadArgs(w)")
				g.l("return")
				g.ind(-1)
				g.l("}")

			}
			returnValueNames := []string{}
			for retI, retField := range method.Return {
				retArgName := retField.Name
				if len(retArgName) == 0 {
					retArgName = "ret"
					if retI > 0 {
						retArgName += "_" + fmt.Sprint(retI)
					}
				}
				returnValueNames = append(returnValueNames, lcfirst(method.Name)+ucfirst(retArgName))
			}
			if len(returnValueNames) > 0 {
				g.app(strings.Join(returnValueNames, ", ") + " := ")
			}
			if isSessionRequest {
				callArgs = append([]string{"w", "r"}, callArgs...)
			}
			g.app("p.service." + method.Name + "(" + strings.Join(callArgs, ", ") + ")")
			g.nl()
			g.l("gotsrpc.Reply([]interface{}{" + strings.Join(returnValueNames, ", ") + "}, w)")
			g.l("return")
			g.ind(-1)
		}
		g.l("default:")
		g.ind(1).l("http.Error(w, \"404 - not found \" + r.URL.Path, http.StatusNotFound)")
		g.ind(-2).l("}") // close switch
		g.ind(-1).l("}") // close ServeHttp

	}
	return nil
}

func RenderGo(services []*Service, longPackageName, packageName string) (gocode string, err error) {
	g := newCode()
	err = renderServiceProxies(services, longPackageName, packageName, g)
	if err != nil {
		return
	}
	gocode = g.string()
	return
}
