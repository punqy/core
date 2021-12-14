package core

import (
	"fmt"
	"runtime"
	"strings"
)

func GetCurrentFuncName() string {
	pc, _, _, _ := runtime.Caller(1)
	parts := strings.Split(runtime.FuncForPC(pc).Name(), "/")
	return fmt.Sprintf("%s", parts[len(parts)-1])
}
