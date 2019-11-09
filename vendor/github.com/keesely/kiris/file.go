package kiris

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	KIRIS_FILE_APPEND = os.O_APPEND | os.O_WRONLY
)

// 获取文件内容
func FileGetContents(file string) ([]byte, error) {
	fl, err := os.Open(file)

	if err != nil {
		return nil, err
	}

	defer fl.Close()

	content, err := ioutil.ReadAll(fl)

	return content, err
}

// 写入文件内容
func FilePutContents(file, content string, opts int) error {
	if true != FileExists(file) {
		_, err := os.Create(file)

		if err != nil {
			return err
		}
	}

	//opts := 0
	fmt.Println("OPTS: ", opts)
	//if opt == KIRIS_FILE_APPEND {
	//opts = os.O_APPEND | os.O_WRONLY
	//}
	if 0 == opts {
		opts = os.O_WRONLY
	}

	fl, err := os.OpenFile(file, opts, 0755)

	n, err := io.WriteString(fl, content)
	defer fl.Close()

	if err != nil {
		return err
	}
	if n > len(content) {
		fmt.Errorf("File put contents error N > len(content)\n")
	}
	return nil
}

// 写入新的文件内容-覆盖
func FilePut(file, content string) error {
	return FilePutContents(file, content, 0)
}

// 追加写入文件内容
func FileAppend(file, content string) error {
	return FilePutContents(file, content, KIRIS_FILE_APPEND)
}

// 检索文件 - 支持通配符
// 例：FileSearch("~/.ssh/*.pub")
func FileSearch(fileName string) ([]string, error) {
	pattern := fileName

	if strings.Index(fileName, "*") != -1 {
		fileName, pattern = getPattern(fileName)
	}

	var list []string
	fileName = RealPath(fileName)

	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if !fi.IsDir() {
		list = append(list, fileName)
		return list, nil
	}

	reg := regexp.MustCompile(pattern)

	// 遍历目录
	filepath.Walk(fileName,
		func(path string, f os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if f.IsDir() {
				path = filepath.ToSlash(path)
				if !strings.HasSuffix(path, "/") {
					path += "/"
				}
				return nil
			}
			// 匹配目录
			pAName := strings.Replace(path, fileName, "", -1)
			matched := reg.MatchString(pAName)
			if matched {
				pflag := true
				match_maps := reg.FindAllStringSubmatch(pAName, -1)
				for k, mms := range match_maps[0] {
					if k <= 0 {
						continue
					}
					smm := fmt.Sprintf("%v", mms)

					if strings.Index(string(smm), "/") != -1 {
						pflag = false
					}
				}
				if pflag == true {
					list = append(list, path)
				}
			}
			return nil
		})

	return list, nil
}

func getPattern(fileName string) (string, string) {
	part_1 := strings.Index(fileName, "*")
	fn_1 := SubStr(fileName, 0, part_1)

	part_2 := strings.LastIndex(fn_1, "/") + 1
	fn_2 := SubStr(fn_1, 0, part_2)

	fn_2_last := SubStr(fn_1, part_2, len(fn_2))

	fn_last := SubStr(fileName, part_1, len(fileName))

	pattern := fn_2_last + fn_last
	fileName = fn_2

	if strings.Index(pattern, ".") != -1 {
		pattern = strings.Replace(pattern, ".", `\.`, -1)
	}

	pattern = strings.Replace(pattern, "*", `(.*)`, -1)
	pattern = "^" + pattern + "$"
	pattern = "(?U)" + pattern

	return fileName, pattern
}
