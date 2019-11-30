// zip.go kee > 2019/11/30

package src

import (
	"archive/zip"
	"assh/src/log"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

//压缩文件
func Zip(src string, dest string) (err error) {
	localFiles, err := ioutil.ReadDir(src)
	if err != nil {
		log.Println(err)
	}

	fzip, err := os.Create(dest)
	if err != nil {
		return
	}
	defer fzip.Close()

	w := zip.NewWriter(fzip)
	defer w.Close()

	for _, backupDir := range localFiles {
		f, ferr := os.Open(path.Join(src, backupDir.Name()))
		if ferr != nil {
			err = ferr
			return
		}
		err = compress(f, ".", w)
		if err != nil {
			return
		}
	}
	return
}

// 批量文件压缩
func Compress(files []*os.File, dest string) error {
	d, _ := os.Create(dest)
	defer d.Close()
	w := zip.NewWriter(d)
	defer w.Close()
	for _, file := range files {
		err := compress(file, "", w)
		if err != nil {
			return err
		}
	}
	return nil
}

func compress(file *os.File, prefix string, zw *zip.Writer) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		prefix = prefix + "/" + info.Name()
		fileInfos, err := file.Readdir(-1)
		if err != nil {
			return err
		}
		for _, fi := range fileInfos {
			f, err := os.Open(file.Name() + "/" + fi.Name())
			if err != nil {
				return err
			}
			err = compress(f, prefix, zw)
			if err != nil {
				return err
			}
		}
	} else {
		header, err := zip.FileInfoHeader(info)
		header.Name = prefix + "/" + header.Name
		if err != nil {
			return err
		}
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, file)
		file.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

//解压
func Unzip(zipFile, dest string) (err error) {
	//目标文件夹不存在则创建
	if _, err = os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(dest, 0755)
		}
	}

	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}

	defer reader.Close()

	for _, file := range reader.File {
		//    log.Println(file.Name)

		if file.FileInfo().IsDir() {

			err := os.MkdirAll(dest+"/"+file.Name, 0755)
			if err != nil {
				log.Println(err)
			}
			continue
		} else {

			err = os.MkdirAll(getDir(dest+"/"+file.Name), 0755)
			if err != nil {
				return err
			}
		}

		rc, err := file.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		filename := dest + "/" + file.Name
		//err = os.MkdirAll(getDir(filename), 0755)
		//if err != nil {
		//    return err
		//}

		w, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer w.Close()

		_, err = io.Copy(w, rc)
		if err != nil {
			return err
		}
		//w.Close()
		//rc.Close()
	}
	return
}

func getDir(path string) string {
	return subString(path, 0, strings.LastIndex(path, "/"))
}

func subString(str string, start, end int) string {
	rs := []rune(str)
	length := len(rs)

	if start < 0 || start > length {
		panic("start is wrong")
	}

	if end < start || end > length {
		panic("end is wrong")
	}

	return string(rs[start:end])
}
