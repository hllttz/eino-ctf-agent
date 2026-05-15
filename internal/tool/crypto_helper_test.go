package tool

import (
	"strings"
	"testing"
)

// ─── hash_identify 测试 ───

func TestCryptoHelper_HashIdentify_MD5(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "hash_identify",
		Text: "d41d8cd98f00b204e9800998ecf8427e",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Result, "MD5") {
		t.Errorf("should identify as MD5, got: %s", out.Result)
	}
}

func TestCryptoHelper_HashIdentify_SHA1(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "hash_identify",
		Text: "da39a3ee5e6b4b0d3255bfef95601890afd80709",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Result, "SHA1") {
		t.Errorf("should identify as SHA1, got: %s", out.Result)
	}
}

func TestCryptoHelper_HashIdentify_SHA256(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "hash_identify",
		Text: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Result, "SHA256") {
		t.Errorf("should identify as SHA256, got: %s", out.Result)
	}
}

func TestCryptoHelper_HashIdentify_SHA512(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "hash_identify",
		Text: "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Result, "SHA512") {
		t.Errorf("should identify as SHA512, got: %s", out.Result)
	}
}

func TestCryptoHelper_HashIdentify_NTLM(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	// NTLM 和 MD5 长度相同，都会匹配
	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "hash_identify",
		Text: "a4f49c525634748239fab4e8b2b14a96",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Result, "MD5") && !strings.Contains(out.Result, "NTLM") {
		t.Errorf("should match at least MD5 or NTLM, got: %s", out.Result)
	}
}

func TestCryptoHelper_HashIdentify_CryptPrefix(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	tests := []struct {
		text     string
		expected string
	}{
		{"$1$abcdefgh$ABCDEFGHIJKLMNOPQRSTUVWXYZ012", "MD5Crypt"},
		{"$5$abcdefgh$ABCDEFGHIJKLMNOPQRSTUVWXYZ012", "SHA256Crypt"},
		{"$6$abcdefgh$ABCDEFGHIJKLMNOPQRSTUVWXYZ012", "SHA512Crypt"},
		{"$2a$10$abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUV", "BlowfishCrypt"},
	}

	for _, tc := range tests {
		out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
			Mode: "hash_identify",
			Text: tc.text,
		})
		if !strings.Contains(out.Result, tc.expected) {
			t.Errorf("text %q should contain %q, got: %s", tc.text[:20], tc.expected, out.Result)
		}
	}
}

func TestCryptoHelper_HashIdentify_Unknown(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "hash_identify",
		Text: "not-a-hash!!!",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Result, "no exact match") && !strings.Contains(out.Result, "length") {
		t.Errorf("should report no exact match, got: %s", out.Result)
	}
}

// ─── base64/base32/hex 解码测试 ───

func TestCryptoHelper_BaseDecode_Hex(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "base_decode",
		Text: "48656c6c6f",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Result, "[hex]") || !strings.Contains(out.Result, "Hello") {
		t.Errorf("should decode hex to 'Hello', got: %s", out.Result)
	}
}

func TestCryptoHelper_BaseDecode_Base64(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "base_decode",
		Text: "SGVsbG8gV29ybGQ=",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Result, "[base64]") || !strings.Contains(out.Result, "Hello World") {
		t.Errorf("should decode base64 to 'Hello World', got: %s", out.Result)
	}
}

func TestCryptoHelper_BaseDecode_Base64NoPadding(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	// "flag" without padding
	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "base_decode",
		Text: "ZmxhZw",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Result, "flag") {
		t.Errorf("should decode base64 to 'flag' (missing padding), got: %s", out.Result)
	}
}

func TestCryptoHelper_BaseDecode_Base32(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "base_decode",
		Text: "JBSWY3DP",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !strings.Contains(out.Result, "[base32]") {
		t.Errorf("should identify as base32, got: %s", out.Result)
	}
}

// ─── XOR 单字节爆破测试 ───

func TestCryptoHelper_XorSingle(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	// "Hello" XOR 0x41 = 09052d2d2e (hex)
	// Actually: H=0x48 ^ 0x41=0x09, e=0x65 ^ 0x41=0x24, etc.
	plaintext := "Hello"
	key := byte(0x41) // 'A'
	encrypted := make([]byte, len(plaintext))
	for i := 0; i < len(plaintext); i++ {
		encrypted[i] = plaintext[i] ^ key
	}
	hexEncrypted := hexEncode(encrypted)

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode:       "xor_single",
		Text:       hexEncrypted,
		MaxResults: 5,
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if len(out.Candidates) == 0 {
		t.Fatal("should have candidates")
	}

	// top candidate should be our plaintext
	found := false
	for _, c := range out.Candidates {
		if c.Plaintext == plaintext && c.Key == "0x41" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("top candidates should include %q with key 0x41, got %d candidates", plaintext, len(out.Candidates))
		for i, c := range out.Candidates {
			t.Logf("  [%d] plain=%q key=%s score=%.3f", i, c.Plaintext, c.Key, c.Score)
		}
	}
}

func TestCryptoHelper_XorSingle_InvalidHex(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "xor_single",
		Text: "not-hex!!!",
	})
	if out.Error == "" {
		t.Fatal("should error on invalid hex input")
	}
	if !strings.Contains(out.Error, "hex-encoded") {
		t.Errorf("error should mention hex-encoded, got: %s", out.Error)
	}
}

func TestCryptoHelper_XorSingle_WikipediaTest(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	// "Wikipedia" XOR 0x10
	plaintext := "Wikipedia"
	key := byte(0x10)
	encrypted := make([]byte, len(plaintext))
	for i := 0; i < len(plaintext); i++ {
		encrypted[i] = plaintext[i] ^ key
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode:       "xor_single",
		Text:       hexEncode(encrypted),
		MaxResults: 10,
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}

	for _, c := range out.Candidates {
		if c.Plaintext == plaintext {
			return // success
		}
	}
	t.Error("top candidates should include 'Wikipedia'")
	for i, c := range out.Candidates {
		t.Logf("  [%d] plain=%q key=%s score=%.4f", i, c.Plaintext, c.Key, c.Score)
	}
}

// ─── XOR repeat 测试 ───

func TestCryptoHelper_XorRepeat_WithKey(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	plaintext := "Hello World"
	key := []byte{0x1a, 0x2b}
	encrypted := xorRepeatingKey([]byte(plaintext), key)

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "xor_repeat",
		Text: hexEncode(encrypted),
		Key:  "1a2b",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.Result != plaintext {
		t.Errorf("should decrypt to %q, got %q", plaintext, out.Result)
	}
}

// ─── Caesar/ROT 测试 ───

func TestCryptoHelper_Caesar_ROT13(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "caesar",
		Text: "Uryyb Jbeyq",
		Key:  "13",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.Result != "Hello World" {
		t.Errorf("ROT13(Uryyb Jbeyq) = %q, want 'Hello World'", out.Result)
	}
}

func TestCryptoHelper_Caesar_BruteForce(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	// "HELLO" with shift 3 = "KHOOR"
	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode:       "caesar",
		Text:       "KHOOR",
		MaxResults: 10,
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if len(out.Candidates) == 0 {
		t.Fatal("should have candidates")
	}

	found := false
	for _, c := range out.Candidates {
		if c.Plaintext == "HELLO" && c.Key == "3" {
			found = true
			break
		}
	}
	if !found {
		t.Error("candidates should include HELLO with shift 3")
		for i, c := range out.Candidates {
			t.Logf("  [%d] plain=%q key=%s score=%.4f", i, c.Plaintext, c.Key, c.Score)
		}
	}
}

func TestCryptoHelper_Caesar_InvalidShift(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "caesar",
		Text: "hello",
		Key:  "999",
	})
	if out.Error == "" {
		t.Fatal("should error on invalid shift")
	}
}

// ─── URL/Hex/Binary decode 测试 ───

func TestCryptoHelper_URLDecode(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "url_decode",
		Text: "hello%20world%21",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.Result != "hello world!" {
		t.Errorf("got %q, want 'hello world!'", out.Result)
	}
}

func TestCryptoHelper_HexDecode(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "hex_decode",
		Text: "666c6167",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.Result != "flag" {
		t.Errorf("got %q, want 'flag'", out.Result)
	}
}

func TestCryptoHelper_BinaryDecode(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "binary_decode",
		Text: "01001000 01101001",
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if out.Result != "Hi" {
		t.Errorf("got %q, want 'Hi'", out.Result)
	}
}

// ─── 错误输入测试 ───

func TestCryptoHelper_EmptyText(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	modes := []string{"hash_identify", "base_decode", "xor_single", "caesar", "url_decode", "hex_decode", "binary_decode"}
	for _, mode := range modes {
		out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
			Mode: mode,
			Text: "",
		})
		if out.Error == "" {
			t.Errorf("mode %s should error on empty text", mode)
		}
	}
}

func TestCryptoHelper_InvalidMode(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode: "not_a_real_mode",
		Text: "test",
	})
	if out.Error == "" {
		t.Fatal("should error on invalid mode")
	}
	if !strings.Contains(out.Error, "unsupported mode") {
		t.Errorf("error should mention unsupported mode, got: %s", out.Error)
	}
}

func TestCryptoHelper_MaxResultsClamping(t *testing.T) {
	tool, err := NewCryptoHelperTool()
	if err != nil {
		t.Fatalf("NewCryptoHelperTool: %v", err)
	}

	// MaxResults > 10 should be clamped to 10 (caesar has 26 shifts)
	out := invokeTool[CryptoHelperInput, CryptoHelperOutput](t, tool, CryptoHelperInput{
		Mode:       "caesar",
		Text:       "test",
		MaxResults: 100,
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if len(out.Candidates) > 10 {
		t.Errorf("max candidates should be clamped to 10, got %d", len(out.Candidates))
	}
}

// ─── readabilityScore 测试 ───

func TestReadabilityScore_English(t *testing.T) {
	engScore := readabilityScore("Hello World this is a test")
	gibScore := readabilityScore("\x01\x02\x03\x04\x05\x06\x07\x08")

	if engScore <= gibScore {
		t.Errorf("English text score (%.4f) should be > garbage score (%.4f)", engScore, gibScore)
	}
}

func TestReadabilityScore_Empty(t *testing.T) {
	if s := readabilityScore(""); s != 0 {
		t.Errorf("empty text score should be 0, got %.4f", s)
	}
}

// ─── helpers ───

func hexEncode(data []byte) string {
	result := make([]byte, len(data)*2)
	for i, b := range data {
		result[i*2] = "0123456789abcdef"[b>>4]
		result[i*2+1] = "0123456789abcdef"[b&0x0f]
	}
	return string(result)
}
