// from https://github.com/qiniu/qshell

package qiniu

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/qiniu/api.v7/storage"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"
)

const (
	needEscape = 0xff
	dontEscape = 16
)

const (
	escapeChar = '\''
)

func genEncoding() []byte {
	var encoding [256]byte
	for c := 0; c <= 0xff; c++ {
		encoding[c] = needEscape
	}
	for c := 'a'; c <= 'f'; c++ {
		encoding[c] = byte(c - ('a' - 10))
	}
	for c := 'A'; c <= 'F'; c++ {
		encoding[c] = byte(c - ('A' - 10))
	}
	for c := 'g'; c <= 'z'; c++ {
		encoding[c] = dontEscape
	}
	for c := 'G'; c <= 'Z'; c++ {
		encoding[c] = dontEscape
	}
	for c := '0'; c <= '9'; c++ {
		encoding[c] = byte(c - '0')
	}
	for _, c := range []byte{'-', '_', '.', '~', '*', '(', ')', '$', '&', '+', ',', ':', ';', '=', '@'} {
		encoding[c] = dontEscape
	}
	encoding['/'] = '!'
	return encoding[:]
}

var encoding = genEncoding()

func encode(v string) string {
	n := 0
	hasEscape := false
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch encoding[c] {
		case needEscape:
			n++
		case '!':
			hasEscape = true
		}
	}
	if !hasEscape && n == 0 {
		return v
	}

	t := make([]byte, len(v)+2*n)
	j := 0
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch encoding[c] {
		case needEscape:
			t[j] = escapeChar
			t[j+1] = "0123456789ABCDEF"[c>>4]
			t[j+2] = "0123456789ABCDEF"[c&15]
			j += 3
		case '!':
			t[j] = encoding[c]
			j++
		default:
			t[j] = c
			j++
		}
	}
	return string(t)
}

func decode(s string) (v string, err error) {
	n := 0
	hasEscape := false
	for i := 0; i < len(s); {
		switch s[i] {
		case escapeChar:
			n++
			if i+2 >= len(s) || encoding[s[i+1]] >= 16 || encoding[s[i+2]] >= 16 {
				return "", syscall.EINVAL
			}
			i += 3
		case '!':
			hasEscape = true
			i++
		default:
			i++
		}
	}
	if !hasEscape && n == 0 {
		return s, nil
	}

	t := make([]byte, len(s)-2*n)

	j := 0
	for i := 0; i < len(s); {
		switch s[i] {
		case escapeChar:
			t[j] = (encoding[s[i+1]] << 4) | encoding[s[i+2]]
			i += 3
		case '!':
			t[j] = '/'
			i++
		default:
			t[j] = s[i]
			i++
		}
		j++
	}
	return string(t), nil
}

// 获取reader中行数
func GetLineCount(reader io.Reader) (totalCount int64) {
	bScanner := bufio.NewScanner(reader)
	for bScanner.Scan() {
		totalCount += 1
	}
	return
}

// 获取文件行数
func GetFileLineCount(filePath string) (totalCount int64) {
	fp, openErr := os.Open(filePath)
	if openErr != nil {
		return
	}
	defer fp.Close()

	return GetLineCount(fp)
}

// URL:
//	 http://host/url
//	 https://host/url
// Path:
//	 AbsolutePath	(Must start with '/')
//	 Pid:RelPath	(Pid.len = 16)
//	 Id: 			(Id.len = 16)
//	 :LinkId:RelPath
//	 :LinkId
func Encode(uri string) string {

	size := len(uri)
	if size == 0 {
		return ""
	}

	encodedURI := encode(uri)
	if c := uri[0]; c == '/' || c == ':' || (size > 16 && encodedURI[16] == ':') || (size > 5 && (encodedURI[4] == ':' || encodedURI[5] == ':')) {
		return encodedURI
	}
	return "!" + encodedURI
}

// Decode
func Decode(encodedURI string) (uri string, err error) {

	size := len(encodedURI)
	if size == 0 {
		return
	}

	if c := encodedURI[0]; c == '!' || c == ':' || (size > 16 && encodedURI[16] == ':') || (size > 5 && (encodedURI[4] == ':' || encodedURI[5] == ':')) {
		uri, err = decode(encodedURI)
		if err != nil {
			return
		}
		if c == '!' {
			uri = uri[1:]
		}
		return
	}

	b := make([]byte, base64.URLEncoding.DecodedLen(len(encodedURI)))
	n, err := base64.URLEncoding.Decode(b, []byte(encodedURI))
	return string(b[:n]), err
}

func getAkBucketFromUploadToken(token string) (ak, bucket string, err error) {
	items := strings.Split(token, ":")
	if len(items) != 3 {
		err = errors.New("invalid upload token, format error")
		return
	}

	ak = items[0]
	policyBytes, dErr := base64.URLEncoding.DecodeString(items[2])
	if dErr != nil {
		err = errors.New("invalid upload token, invalid put policy")
		return
	}

	putPolicy := storage.PutPolicy{}
	uErr := json.Unmarshal(policyBytes, &putPolicy)
	if uErr != nil {
		err = errors.New("invalid upload token, invalid put policy")
		return
	}

	bucket = strings.Split(putPolicy.Scope, ":")[0]
	return
}

// 从URL中获取文件名字
func KeyFromUrl(uri string) (key string, err error) {
	u, pErr := url.Parse(uri)
	if pErr != nil {
		err = pErr
		return
	}
	for _, c := range u.Path {
		if c != '/' {
			break
		}
		key = u.Path[1:]
	}
	return
}

// 表示大小
type ByteSize int64

const (
	KB ByteSize = 1024
	MB          = 1024 * KB
	GB          = 1024 * MB
	TB          = 1024 * GB
)

func (b ByteSize) String() string {
	if b < KB {
		return strconv.FormatInt(int64(b), 10) + "B"
	}
	if b >= KB && b < MB {
		size := float64(b) / float64(KB)
		return strconv.FormatFloat(size, 'f', 2, 64) + "KB"
	}
	if b >= MB && b < GB {
		size := float64(b) / float64(MB)
		return strconv.FormatFloat(size, 'f', 2, 64) + "MB"
	}
	if b >= GB && b < TB {
		size := float64(b) / float64(GB)
		return strconv.FormatFloat(size, 'f', 2, 64) + "GB"
	}
	size := float64(b) / float64(TB)
	return strconv.FormatFloat(size, 'f', 2, 64) + "TB"
}

// 将字节转化为人工可读的字符串
// b - 表示文件大小，单位字节, readable - 可读字符串
// 比如1304 ==》1304/1024 ==> 1.27KB
func BytesToReadable(size int64) (readable string) {
	return ByteSize(size).String()
}
