package encoder

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// ToBytes 将字符串转换为指定编码的字节数组
// 支持的编码: utf-8, utf-16, utf-16le, utf-16be, gbk, gb2312, big5, euc-jp, shift-jis, euc-kr, iso-8859-1, windows-1252
func ToBytes(s string, encodingName string) ([]byte, error) {
	var enc encoding.Encoding

	if encodingName == "" {
		encodingName = "iso-8859-1"
	}

	// 根据编码名称选择对应的编码器
	switch encodingName {
	case "utf-8":
		// UTF-8是Go的原生编码，直接转换
		return []byte(s), nil
	case "utf-16":
		// UTF-16带BOM
		enc = unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
	case "utf-16le":
		// UTF-16小端序
		enc = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	case "utf-16be":
		// UTF-16大端序
		enc = unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	case "gbk":
		enc = simplifiedchinese.GBK
	case "gb2312":
		enc = simplifiedchinese.HZGB2312
	case "big5":
		enc = traditionalchinese.Big5
	case "euc-jp":
		enc = japanese.EUCJP
	case "shift-jis":
		enc = japanese.ShiftJIS
	case "euc-kr":
		enc = korean.EUCKR
	case "iso-8859-1":
		enc = charmap.ISO8859_1
	case "windows-1252":
		enc = charmap.Windows1252
	default:
		return nil, errors.New("不支持的编码格式: " + encodingName)
	}

	// 进行编码转换
	result, _, err := transform.Bytes(enc.NewEncoder(), []byte(s))
	if err != nil {
		return nil, fmt.Errorf("编码转换失败: %w", err)
	}

	return result, nil
}

// ToString 将字节数组按照指定编码转换为字符串
// 支持的编码: utf-8, utf-16, utf-16le, utf-16be, gbk, gb2312, big5, euc-jp, shift-jis, euc-kr, iso-8859-1, windows-1252
func ToString(b []byte, encodingName string) (string, error) {
	var enc encoding.Encoding

	// 根据编码名称选择对应的解码器
	switch encodingName {
	case "utf-8":
		// UTF-8是Go的原生编码，直接转换
		return string(b), nil
	case "utf-16":
		// UTF-16带BOM
		enc = unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
	case "utf-16le":
		// UTF-16小端序
		enc = unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	case "utf-16be":
		// UTF-16大端序
		enc = unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	case "gbk":
		enc = simplifiedchinese.GBK
	case "gb2312":
		enc = simplifiedchinese.HZGB2312
	case "big5":
		enc = traditionalchinese.Big5
	case "euc-jp":
		enc = japanese.EUCJP
	case "shift-jis":
		enc = japanese.ShiftJIS
	case "euc-kr":
		enc = korean.EUCKR
	case "iso-8859-1":
		enc = charmap.ISO8859_1
	case "windows-1252":
		enc = charmap.Windows1252
	default:
		return "", errors.New("不支持的编码格式: " + encodingName)
	}

	// 进行解码转换
	result, _, err := transform.Bytes(enc.NewDecoder(), b)
	if err != nil {
		return "", fmt.Errorf("解码转换失败: %w", err)
	}

	return string(result), nil
}

// B64Encode 编码字节切片为base64字符串，支持可选的替代字符
// altchars应为长度为2的字符串，分别替代'+'和'/'
func B64Encode(s []byte, altchars string) string {
	// 使用标准base64编码
	encoded := base64.StdEncoding.EncodeToString(s)

	// 如果提供了替代字符，则替换'+'和'/'
	if altchars != "" {
		// 验证替代字符长度
		if len(altchars) != 2 {
			panic("altchars must be a string of length 2")
		}

		// 替换'+'和'/'
		result := strings.ReplaceAll(encoded, "+", string(altchars[0]))
		result = strings.ReplaceAll(result, "/", string(altchars[1]))
		return result
	}

	return encoded
}

var base64Regex = regexp.MustCompile(`^[A-Za-z0-9+/]*={0,2}$`)

// B64Decode 解码base64字符串为字节切片
// s为要解码的base64字符串
// altchars应为长度为2的字符串，指定用于替换'+'和'/'的字符（可选）
// validate为true时，严格验证输入是否符合base64格式
func B64Decode(s string, altchars string, validate bool) ([]byte, error) {
	processed := s

	// 处理替代字符
	if altchars != "" {
		if len(altchars) != 2 {
			return nil, errors.New("altchars must be a string of length 2")
		}
		// 将替代字符换回标准的'+'和'/'
		processed = strings.ReplaceAll(processed, string(altchars[0]), "+")
		processed = strings.ReplaceAll(processed, string(altchars[1]), "/")
	}

	// 添加自动补齐功能：确保长度是4的倍数
	paddingNeeded := 4 - (len(processed) % 4)
	if paddingNeeded < 4 { // 避免添加4个填充符（当长度已是4的倍数时）
		processed += strings.Repeat("=", paddingNeeded)
	}

	// 验证模式处理
	if validate {
		if !base64Regex.MatchString(processed) {
			return nil, errors.New("non-base64 digit found")
		}
	} else {
		// 非验证模式下，移除所有非base64字符
		// 先找到所有匹配的字符，再拼接起来
		//matches := base64Regex.FindAllString(processed, -1)
		//processed = strings.Join(matches, "")
	}

	// 执行base64解码
	return base64.StdEncoding.DecodeString(processed)
}

func StandardB64Encode(s []byte) string {
	return B64Encode(s, "")
}

func StandardB64Decode(s string) []byte {
	res, err := B64Decode(s, "", false)
	if err != nil {
		panic(err)
	}
	return res
}

func UrlSafeB64Encode(s []byte) string {
	return B64Encode(s, "-_")
}

func UrlSafeB64Decode(s string) []byte {
	res, err := B64Decode(s, "-_", false)
	if err != nil {
		panic(err)
	}
	return res
}
