// Package qiniu 提供七牛云存储的上传、下载和文件管理功能。
//
// 基于 qiniu/api.v7 库封装，支持文件上传、下载、Bucket 管理和 URI 编解码。
// 代码源自 https://github.com/qiniu/qshell。
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

// 字符编码状态常量：需要转义或不需要转义。
const (
	needEscape = 0xff // 需要转义的字符
	dontEscape = 16   // 不需要转义的标记
)

const (
	escapeChar = '\'' // 转义字符前缀
)

// genEncoding 生成 URI 编码表，定义每个字节的编码行为。
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

// encode 对字符串进行 Qiniu 自定义 URI 编码（非标准 URL 编码）。
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

// decode 对 Qiniu 自定义编码的字符串进行解码。
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

// GetLineCount 统计读取器中的行数。
func GetLineCount(reader io.Reader) (totalCount int64) {
	bScanner := bufio.NewScanner(reader)
	for bScanner.Scan() {
		totalCount += 1
	}
	return
}

// GetFileLineCount 统计文件的总行数。
func GetFileLineCount(filePath string) (totalCount int64) {
	fp, openErr := os.Open(filePath)
	if openErr != nil {
		return
	}
	defer fp.Close()

	return GetLineCount(fp)
}

// Encode 对 URI 进行编码。
// 支持的输入格式：
//   - URL: http://host/path, https://host/path
//   - 绝对路径: 以 '/' 开头
//   - 资源路径: Pid:RelPath (Pid 长度 16)
//   - 资源 ID: Id (Id 长度 16)
//   - 链接路径: :LinkId:RelPath, :LinkId
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

// Decode 对 Qiniu 编码的 URI 进行解码，支持 Base64 URL 编码和自定义编码。
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

// getAkBucketFromUploadToken 从上传令牌中解析 AccessKey 和存储空间名。
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

// KeyFromUrl 从 URL 中提取文件名（最后一段路径）。
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

// ByteSize 表示文件大小，支持自动转换为人类可读的字符串格式。
type ByteSize int64

const (
	KB ByteSize = 1024       // 千字节
	MB          = 1024 * KB  // 兆字节
	GB          = 1024 * MB  // 吉字节
	TB          = 1024 * GB  // 太字节
)

// String 将文件大小转换为人类可读的字符串表示。
// 例如：1304 -> "1.27KB"。
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

// BytesToReadable 将字节数转换为人类可读的字符串。
// 例如：BytesToReadable(1304) 返回 "1.27KB"。
func BytesToReadable(size int64) (readable string) {
	return ByteSize(size).String()
}
