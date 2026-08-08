package auth

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"
)

// 用户资料字段校验器。纯函数、无副作用、不依赖 env/外部服务。
// 校验失败返回对应 sentinel error，供 service 层映射为 400。
var (
	ErrBadUsername = errors.New("用户名需 3-20 位，小写字母开头，仅含小写字母/数字/下划线")
	ErrBadPassword = errors.New("密码需 8-64 位，且同时包含字母和数字，不能有空格或中文")
	ErrBadEmail    = errors.New("邮箱格式不正确")
	ErrBadNickname = errors.New("昵称需 1-30 个字符")
	ErrBadAge      = errors.New("年龄需在 0-150 之间")
	ErrBadGender   = errors.New("性别只能是 男/女/其他 或留空")
)

const (
	nicknameMinRunes = 1
	nicknameMaxRunes = 30
	passwordMin      = 8
	passwordMax      = 64
	ageMin           = 0
	ageMax           = 150
)

// usernameRe：小写字母开头，后接 2-19 位小写字母/数字/下划线，总长 3-20。
var usernameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{2,19}$`)

// ValidateUsername 校验用户名格式。
func ValidateUsername(s string) error {
	if !usernameRe.MatchString(s) {
		return ErrBadUsername
	}
	return nil
}

// ValidatePassword 校验密码：8-64 位、仅 ASCII 可见字符(33-126)、
// 同时含字母和数字、无空格/中文/控制字符。
func ValidatePassword(s string) error {
	if len(s) < passwordMin || len(s) > passwordMax {
		return ErrBadPassword
	}
	var hasLetter, hasDigit bool
	for _, r := range s {
		if r < 33 || r > 126 {
			// 空格(32)、控制字符、中文等非 ASCII 可见字符一律拒绝。
			return ErrBadPassword
		}
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return ErrBadPassword
	}
	return nil
}

// NormalizeEmail 校验并规范化邮箱：去空白、用 net/mail.ParseAddress 解析、小写化。
// 返回规范化后的地址；格式非法返回 ErrBadEmail。
func NormalizeEmail(s string) (string, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", ErrBadEmail
	}
	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return "", ErrBadEmail
	}
	return strings.ToLower(addr.Address), nil
}

// NormalizeNickname 去除首尾空白；为空则使用 fallback(通常是 username)。
// rune 计数需在 1-30 之间，超长返回 ErrBadNickname。
func NormalizeNickname(s, fallback string) (string, error) {
	name := strings.TrimSpace(s)
	if name == "" {
		name = fallback
	}
	n := utf8.RuneCountInString(name)
	if n < nicknameMinRunes || n > nicknameMaxRunes {
		return "", ErrBadNickname
	}
	return name, nil
}

// ValidateAge 校验年龄在 0-150 之间。
func ValidateAge(n int) error {
	if n < ageMin || n > ageMax {
		return ErrBadAge
	}
	return nil
}

// ValidateGender 校验性别只能是 男/女/其他 或留空。
func ValidateGender(s string) error {
	switch s {
	case "男", "女", "其他", "":
		return nil
	default:
		return ErrBadGender
	}
}
