package los

import (
	"fmt"
	"os"
	"strings"
)

const (
	OSWindows = "windows"
	OSLinux   = "linux"
	OSDarwin  = "darwin"

	WinPathSep = `\`
	NixPathSep = "/"

	NixEnvVarSep  = ":"
	NixPathEnvVar = "PATH"

	WinEnvVarSep  = ";"
	WinPathEnvVar = "Path"
)

var (
	OS string

	PathSep string

	ExecExt    string
	LibraryExt string

	// Environment variable related
	EnvVarSep  string
	PathEnvVar string
)

func IsKnownOS(os string) bool {
	return os == OSWindows || os == OSLinux || os == OSDarwin
}

func GetPathEnv() string {
	return os.Getenv(PathEnvVar)
}

func BuildPathEnv(value ...string) string {
	return fmt.Sprintf("%s=%s", PathEnvVar, strings.Join(value, EnvVarSep))
}

func TrimPathSep(path string) string {
	return strings.TrimRight(path, PathSep)
}
