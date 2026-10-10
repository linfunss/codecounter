package format

import "testing"

// TestInt 验证千位分隔符的处理。
func TestInt(t *testing.T) {
	cases := map[int64]string{
		0:          "0",
		7:          "7",
		999:        "999",
		1000:       "1,000",
		1234567:    "1,234,567",
		-1234:      "-1,234",
		1000000000: "1,000,000,000",
	}

	for in, want := range cases {
		if got := Int(in); got != want {
			t.Errorf("Int(%d) = %q, 期望 %q", in, got, want)
		}
	}
}

// TestBytes 验证容量单位的换算。
func TestBytes(t *testing.T) {
	cases := map[int64]string{
		0:          "0 B",
		512:        "512 B",
		1024:       "1.0 KiB",
		1536:       "1.5 KiB",
		1048576:    "1.0 MiB",
		1073741824: "1.0 GiB",
	}

	for in, want := range cases {
		if got := Bytes(in); got != want {
			t.Errorf("Bytes(%d) = %q, 期望 %q", in, got, want)
		}
	}
}
