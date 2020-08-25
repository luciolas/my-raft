package service

import (
	"fmt"
	"reflect"
	"strings"
)

type Service struct {
	Name          string
	Rcvr          reflect.Value
	Typ           reflect.Type
	Methods       map[string]reflect.Method
	SvcMethodName []string
}

func MakeService(rcvr interface{}, maxDepth int) map[string]*Service {
	var svces map[string]*Service = make(map[string]*Service)

	var ss []chan reflect.Value = make([]chan reflect.Value, maxDepth+1)
	for i := 0; i < maxDepth+1; i++ {
		var sss chan reflect.Value = make(chan reflect.Value, 16)
		ss[i] = sss
	}
	currDepth := 0
	ss[currDepth] <- reflect.ValueOf(rcvr)
	for {
		fmt.Println("-------------------------")
		if currDepth > maxDepth {
			break
		}
		nextDepth := currDepth + 1
		depth := ss[currDepth]
		var pvrc reflect.Value
		for {
			if len(depth) == 0 {
				currDepth++
				break
			}
			pvrc = <-depth
			vrc := reflect.Indirect(pvrc)
			fmt.Printf("vcr: %s\n", vrc.Type().Name())
			if vrc.Kind() == reflect.Struct {
				// Push fields which are structs in the parent struct
				nfields := vrc.NumField()
				if nextDepth <= maxDepth {
					for i := 0; i < nfields; i++ {
						if vField := vrc.Field(i); reflect.Indirect(vField).Kind() == reflect.Struct {
							ss[nextDepth] <- vField
							fmt.Printf("vField: %s\n", vField.Type().Name())
						}
					}
				}
				// Analyse methods of current value
				methods := map[string]reflect.Method{}
				pvrctype := pvrc.Type()
				methodFullNames := make([]string, pvrctype.NumMethod())

				fmt.Printf("method count: %d\n", pvrc.NumMethod())
				for i := 0; i < pvrctype.NumMethod(); i++ {
					vrcmethod := pvrctype.Method(i)
					vrcmethodtype := vrcmethod.Type
					if vrcmethod.PkgPath != "" || // capitalized?
						vrcmethodtype.NumIn() != 3 ||
						vrcmethodtype.In(2).Kind() != reflect.Ptr ||
						vrcmethodtype.NumOut() != 0 {
						// fmt.Printf("No method: %s \n", vrcmethodtype.Name())
					} else {
						// the method looks like a handler
						methods[vrcmethod.Name] = vrcmethod
						methodFullNames = append(methodFullNames, strings.Join([]string{vrc.Type().Name(), vrcmethod.Name}, "."))
						fmt.Printf("method: %s \n", vrcmethod.Name)
					}
				}
				if len(methods) > 0 {
					vService := &Service{}
					vService.Methods = methods
					vService.Rcvr = pvrc
					vService.Name = vrc.Type().Name()
					vService.Typ = vrc.Type()
					vService.SvcMethodName = methodFullNames
					svces[vService.Name] = vService
				}
			}
		}

	}
	return svces
}
