package utils

import (
	"math"
	"testing"
)

func Test_GenerateRandomBytes(t *testing.T) {
	t.Parallel()
	length := 1024
	data, err := GenerateRandomBytes(length)
	Assert(t, err, nil, "error")
	Assert(t, len(data), length, "output length")
}

func Test_Ternary(t *testing.T) {
	t.Parallel()
	for i := 0; i < 100; i++ {
		out, ok := Ternary(i%2 != 0, true, false).(bool)
		Assert(t, ok, true, "returned value type")
		if i%2 != 0 {
			Assert(t, out, true, "return value")
		} else {
			Assert(t, out, false, "return value")
		}
	}
}

func Test_IsPowerOfTwo(t *testing.T) {
	t.Parallel()
	Assert(t, IsPowerOfTwo(0), false, "0")
	Assert(t, IsPowerOfTwo(1), true, "1")
	Assert(t, IsPowerOfTwo(int64(math.Pow(2, 32))), true, "2^12")
	Assert(t, IsPowerOfTwo(int64(math.Pow(3, 32))), false, "3^32")
	Assert(t, IsPowerOfTwo(int64(math.Pow(4, 20))), true, "4^20")
	Assert(t, IsPowerOfTwo(int64(math.Pow(5, 12))), false, "5^12")
	Assert(t, IsPowerOfTwo(int64(math.Pow(6, 17))), false, "6^17")
}

func Test_GenerateRandomString(t *testing.T) {
	t.Parallel()
	length := 1024
	data, err := GenerateRandomString(length)
	Assert(t, err, nil, "error")
	Assert(t, len(data), length, "output length")
}

func Test_B2S(t *testing.T) {
	t.Parallel()
	raw := "This is a test string"
	Assert(t, B2S([]byte(raw)), raw, "Test string with length")
	Assert(t, B2S([]byte{}), "", "Test string without length")
}

func Test_S2B(t *testing.T) {
	t.Parallel()
	raw := "This is a test string"
	Assert(t, string(S2B(raw)), raw, "Test string with length")
	Assert(t, string(S2B("")), "", "Test string without length")
}

func Test_IsPointer(t *testing.T) {
	t.Parallel()
	raw := "This is a test string"
	Assert(t, IsPointer(raw), false, "Test non pointer")
	Assert(t, IsPointer(&raw), true, "Test pointer")
}

func Test_PointerHelpers(t *testing.T) {
	t.Parallel()
	Assert(t, *StringPointer("123"), "123", "Test string pointer")
	Assert(t, *Int32Pointer(123), int32(123), "Test int32 pointer")
	Assert(t, *Int64Pointer(123), int64(123), "Test int64 pointer")
	Assert(t, *BoolPointer(true), true, "Test bool pointer")
}

func Assert(t *testing.T, value interface{}, expected interface{}, meaning string) {
	if value != expected {
		t.Fatalf("%s, expected %v recieved %v", meaning, expected, value)
	}
}
