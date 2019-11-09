package kiris

import (
	"bytes"
	//"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

func OsStat(path string) (os.FileInfo, error) {
	return os.Stat(RealPath(path))
}

func FileExists(file string) bool {
	_, err := OsStat(file)
	return !os.IsNotExist(err)
}

func IsDir(dir string) bool {
	if s, err := OsStat(dir); err == nil {
		return s.IsDir()
	}
	return false
}

func IsFile(file string) bool {
	return !IsDir(file)
}

func RealPath(path string) string {
	str := []rune(path)
	if "~" == string(str[:1]) {
		if home := Home(); home != "" {
			return home + string(str[1:])
		}
	} else if "/" == string(str[:1]) {
		return path
	}

	abs, e := filepath.Abs(path)
	//fmt.Printf(" >>> PATH : %s => REAL PATH: %s\n", path, abs)
	return Ternary(e == nil, abs, string(CurrentPath()+"/"+path)).(string)
}

func CurrentPath() string {
	if dir, err := filepath.Abs(filepath.Dir(os.Args[0])); err == nil {
		return strings.Replace(dir, "\\", "/", -1)
	}
	return ""
}

func Home() string {
	if u, err := user.Current(); err == nil {
		return u.HomeDir
	}

	if "windows" == runtime.GOOS {
		return windowsHome()
	}

	return unixHome()
}

func windowsHome() string {
	drive := os.Getenv("HOMEDRIVE")
	path := os.Getenv("HOMEPATH")
	home := drive + path

	if drive == "" || path == "" {
		home = os.Getenv("USERPROFILE")
	}
	return home
}

func unixHome() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	var stdout bytes.Buffer
	cmd := exec.Command("sh", "-c", "eval eco ~$USER")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}
	if result := strings.TrimSpace(stdout.String()); result != "" {
		return result
	}
	return ""
}
