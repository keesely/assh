package kiris

import (
	"fmt"
	"math/rand"
	"os"
	"time"
)

/**
 * 处理三元运算
 *
 * @param {bool}  cond  输入运算条件
 * @param {interface{}} Tval 如果符合条件则返回Tval
 * @param {interface{}} Fval 如果条件不符合，则返回Fval
 *
 * @return interface{}
 * */
func Ternary(cond bool, Tval, Fval interface{}) interface{} {
	if cond {
		return Tval
	}
	return Fval
}

/**
 * 处理深度拷贝
 *
 * @param {interface{}} value 需要深拷贝的值
 *
 * @return interface{}
 * */
func DeepCopy(value interface{}) interface{} {
	if valueMap, ok := value.(map[string]interface{}); ok {
		newMap := make(map[string]interface{})
		for k, v := range valueMap {
			newMap[k] = DeepCopy(v)
		}

		return newMap
	} else if valueSlice, ok := value.([]interface{}); ok {
		newSlice := make([]interface{}, len(valueSlice))
		for k, v := range valueSlice {
			newSlice[k] = DeepCopy(v)
		}

		return newSlice
	}

	return value
}

/**
 * 获取系统环境变量
 *
 * @param {string}  key 环境变量key
 * @param {interface{}} def 如果不存在则返回默认, 缺省为nil
 *
 * @return interface{}
 * */
func GetEnv(key string, def ...interface{}) interface{} {
	var _def interface{}
	if def != nil && def[0] != nil {
		_def = def[0]
	}

	val := os.Getenv(key)
	return Ternary(val != "", val, _def)
}

/**
 * 获取数据类型名称
 *
 * @return string
 * */
func Typeof(value interface{}) string {
	return fmt.Sprintf("%T", value)
}

/**
 * 生成指定区间随机数
 * @parans int  x 起始区间
 * @params int  y 结束区间
 *
 * @return int
 * */
func Rand(x, y int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(y-x+1) + x
}

const (
	KIRIS_STR_PAD_LEFT  = 0
	KIRIS_STR_PAD_RIGHT = 1
	KIRIS_STR_PAD_BOTH  = 2
)

/**
 * StrPad 定义把规定字符串填充为新的长度
 *
 * @param {string} str     要填充的字符串
 * @param {string} pad     提供填充的字符串
 * @param {int}    length  规定填充的字符串的长度
 * @param {int}    padType 规定填充位置
 * 预置常量
 * - KIRIS_STR_PAD_LEFT  = 0 填充字符串左侧
 * - KIRIS_STR_PAD_RIGHT = 1 填充字符串右侧
 * - KIRIS_STR_PAD_BOTH  = 2 填充字符串两侧, 如果填充不是偶数，则右侧获得额外填充
 *
 * @return string
 * */
func StrPad(str, pad string, length, padType int) string {
	LEN := StrCount(str)

	if LEN >= length {
		return str
	}

	half := 0
	if 2 == padType {
		half = (length - LEN) / 2
	}

	l_pad := ""
	r_pad := ""

	_pad := []rune(pad)
	for i := LEN; i < length; {
		for _, p := range _pad {
			i++
			if i > length {
				break
			}

			s := string(p)
			if 0 < padType {
				r_pad += s
				if KIRIS_STR_PAD_BOTH == padType && 0 < half && i < length {
					i++
					l_pad += s
				}
			} else if 0 == padType {
				l_pad += s
			}
		}
	}

	return l_pad + str + r_pad
}

/**
 * 获取字符串字数
 * @param {string} str 要计算的字符串
 *
 * @return int
 * */
func StrCount(str string) int {
	return len([]rune(str))
}

//截取字符串
func SubStr(str string, pos, length int) string {
	runes := []rune(str)
	l := pos + length
	l = Ternary(l > len(runes), len(runes), l).(int)
	return string(runes[pos:l])
}
