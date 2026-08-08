package auth

import (
	"errors"
	"testing"
)

func TestValidateUsername(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		{"太短", "ab", ErrBadUsername},
		{"大写", "Abc", ErrBadUsername},
		{"数字开头", "1ab", ErrBadUsername},
		{"含空格", "a b", ErrBadUsername},
		{"中文", "中文", ErrBadUsername},
		{"太长(21位)", "a12345678901234567890", ErrBadUsername},
		{"非法符号", "a-bc", ErrBadUsername},
		{"合法最短", "a_1", nil},
		{"合法带数字下划线", "abc_123", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateUsername(c.in)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("ValidateUsername(%q) = %v, want %v", c.in, err, c.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		{"太短", "short1", ErrBadPassword},
		{"无数字", "nodigitsss", ErrBadPassword},
		{"无字母", "12345678", ErrBadPassword},
		{"含空格", "has space1", ErrBadPassword},
		{"含中文", "密码密码abcd1", ErrBadPassword},
		{"含控制字符", "abcd1234\t", ErrBadPassword},
		{"太长(65位)", "a1234567890123456789012345678901234567890123456789012345678901234", ErrBadPassword},
		{"合法", "abcd1234", nil},
		{"合法带符号", "Ab!@#123", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePassword(c.in)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("ValidatePassword(%q) = %v, want %v", c.in, err, c.wantErr)
			}
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr error
	}{
		{"缺@", "x", "", ErrBadEmail},
		{"无点无tld", "a@b", "a@b", nil},
		{"合法小写化", "A@B.COM", "a@b.com", nil},
		{"去空白", "  a@b.com  ", "a@b.com", nil},
		{"空字符串", "", "", ErrBadEmail},
		{"带名字格式", "Foo <a@b.com>", "a@b.com", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NormalizeEmail(c.in)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("NormalizeEmail(%q) err = %v, want %v", c.in, err, c.wantErr)
			}
			if c.wantErr == nil && got != c.want {
				t.Fatalf("NormalizeEmail(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNormalizeNickname(t *testing.T) {
	long31 := "1234567890123456789012345678901"
	cases := []struct {
		name     string
		in       string
		fallback string
		want     string
		wantErr  error
	}{
		{"空→fallback", "", "alice", "alice", nil},
		{"纯空白→fallback", "   ", "alice", "alice", nil},
		{"去首尾空白", "  hi  ", "alice", "hi", nil},
		{"31字符→错", long31, "alice", "", ErrBadNickname},
		{"合法1字符", "喵", "alice", "喵", nil},
		{"30个中文合法", "一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十", "alice", "一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NormalizeNickname(c.in, c.fallback)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("NormalizeNickname(%q,%q) err = %v, want %v", c.in, c.fallback, err, c.wantErr)
			}
			if c.wantErr == nil && got != c.want {
				t.Fatalf("NormalizeNickname(%q,%q) = %q, want %q", c.in, c.fallback, got, c.want)
			}
		})
	}
}

func TestValidateAge(t *testing.T) {
	cases := []struct {
		name    string
		in      int
		wantErr error
	}{
		{"负数", -1, ErrBadAge},
		{"超上限", 151, ErrBadAge},
		{"下限0", 0, nil},
		{"常规30", 30, nil},
		{"上限150", 150, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateAge(c.in)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("ValidateAge(%d) = %v, want %v", c.in, err, c.wantErr)
			}
		})
	}
}

func TestValidateGender(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		{"男", "男", nil},
		{"女", "女", nil},
		{"其他", "其他", nil},
		{"留空", "", nil},
		{"非法x", "x", ErrBadGender},
		{"非法英文", "male", ErrBadGender},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateGender(c.in)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("ValidateGender(%q) = %v, want %v", c.in, err, c.wantErr)
			}
		})
	}
}
