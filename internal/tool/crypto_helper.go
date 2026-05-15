package tool

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// CryptoHelperInput crypto_helper 工具的输入参数。
type CryptoHelperInput struct {
	Mode       string `json:"mode" jsonschema:"description=operation mode: hash_identify, base_decode, xor_single, xor_repeat, caesar, url_decode, hex_decode, binary_decode"`
	Text       string `json:"text" jsonschema:"description=input text to process"`
	Key        string `json:"key,omitempty" jsonschema:"description=key for xor_repeat mode as hex string (e.g. 0a1b); for caesar mode as shift number"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"description=max candidates to return for brute force modes, default 10, max 10"`
}

// CryptoCandidate 爆破模式中的单个候选结果。
type CryptoCandidate struct {
	Plaintext string  `json:"plaintext" jsonschema:"description=decoded/decrypted plaintext"`
	Key       string  `json:"key,omitempty" jsonschema:"description=key for xor modes (hex byte) or caesar (shift number)"`
	Score     float64 `json:"score" jsonschema:"description=readability score, higher is more likely to be correct text"`
}

// CryptoHelperOutput crypto_helper 工具的输出结果。
type CryptoHelperOutput struct {
	Result     string            `json:"result" jsonschema:"description=primary result for non-brute-force modes"`
	Candidates []CryptoCandidate `json:"candidates,omitempty" jsonschema:"description=top candidates for brute force modes (xor_single, xor_repeat, caesar)"`
	Error      string            `json:"error,omitempty" jsonschema:"description=error message if operation failed"`
}

// NewCryptoHelperTool 创建 CTF 密码学辅助工具。
func NewCryptoHelperTool() (einotool.InvokableTool, error) {
	return utils.InferTool[CryptoHelperInput, CryptoHelperOutput](
		"crypto_helper",
		"CTF cryptography helper. Supports hash type identification (MD5/SHA1/SHA256/SHA512/NTLM), "+
			"auto base64/base32/hex decode with padding fix, "+
			"single-byte XOR brute force, repeating-key XOR decode, "+
			"Caesar/ROT brute force (shifts 0-25), "+
			"URL/hex/binary decode. "+
			"Use this tool when you encounter hashes, encoded strings, or XOR-encrypted data in CTF challenges. "+
			"For brute force modes, returns top candidates sorted by readability score.",
		func(ctx context.Context, input CryptoHelperInput) (CryptoHelperOutput, error) {
			mode := strings.ToLower(strings.TrimSpace(input.Mode))
			text := input.Text
			log.Printf("[crypto-tool] mode=%s text_prefix=%q", mode, truncateStr(text, 80))
			if text == "" {
				return CryptoHelperOutput{Error: "text is empty"}, nil
			}

			limit := input.MaxResults
			if limit <= 0 || limit > 10 {
				limit = 10
			}

			switch mode {
			case "hash_identify":
				return identifyHash(text), nil
			case "base_decode":
				return autoBaseDecode(text), nil
			case "xor_single":
				return xorSingleBrute(text, limit), nil
			case "xor_repeat":
				return xorRepeatBrute(text, input.Key, limit), nil
			case "caesar":
				return caesarBrute(text, input.Key, limit), nil
			case "url_decode":
				return simpleURLDecode(text), nil
			case "hex_decode":
				return simpleHexDecode(text), nil
			case "binary_decode":
				return simpleBinaryDecode(text), nil
			default:
				return CryptoHelperOutput{Error: fmt.Sprintf(
					"unsupported mode: %s (supported: hash_identify, base_decode, xor_single, xor_repeat, caesar, url_decode, hex_decode, binary_decode)",
					mode)}, nil
			}
		},
	)
}

// ─── hash_identify ───

type hashPattern struct {
	Name   string
	Regex  *regexp.Regexp
	Length int // 0 means no fixed length check
}

var hashPatterns = []hashPattern{
	{Name: "MD5", Regex: regexp.MustCompile(`^[a-fA-F0-9]{32}$`), Length: 32},
	{Name: "SHA1", Regex: regexp.MustCompile(`^[a-fA-F0-9]{40}$`), Length: 40},
	{Name: "SHA256", Regex: regexp.MustCompile(`^[a-fA-F0-9]{64}$`), Length: 64},
	{Name: "SHA512", Regex: regexp.MustCompile(`^[a-fA-F0-9]{128}$`), Length: 128},
	{Name: "NTLM", Regex: regexp.MustCompile(`^[a-fA-F0-9]{32}$`), Length: 32},
	{Name: "SHA224", Regex: regexp.MustCompile(`^[a-fA-F0-9]{56}$`), Length: 56},
	{Name: "SHA384", Regex: regexp.MustCompile(`^[a-fA-F0-9]{96}$`), Length: 96},
	{Name: "MD4", Regex: regexp.MustCompile(`^[a-fA-F0-9]{32}$`), Length: 32},
	{Name: "RIPEMD160", Regex: regexp.MustCompile(`^[a-fA-F0-9]{40}$`), Length: 40},
	{Name: "Whirlpool", Regex: regexp.MustCompile(`^[a-fA-F0-9]{128}$`), Length: 128},
	{Name: "MySQL323", Regex: regexp.MustCompile(`^[a-fA-F0-9]{16}$`), Length: 16},
	{Name: "MySQLSHA1", Regex: regexp.MustCompile(`^\*[a-fA-F0-9]{40}$`), Length: 41},
	{Name: "MD5Crypt", Regex: regexp.MustCompile(`^\$1\$`), Length: 0},
	{Name: "SHA256Crypt", Regex: regexp.MustCompile(`^\$5\$`), Length: 0},
	{Name: "SHA512Crypt", Regex: regexp.MustCompile(`^\$6\$`), Length: 0},
	{Name: "BlowfishCrypt", Regex: regexp.MustCompile(`^\$2[ayb]\$`), Length: 0},
	{Name: "CRC32", Regex: regexp.MustCompile(`^[a-fA-F0-9]{8}$`), Length: 8},
	// CRC16-like
	{Name: "CRC16", Regex: regexp.MustCompile(`^[a-fA-F0-9]{4}$`), Length: 4},
}

func identifyHash(text string) CryptoHelperOutput {
	cleaned := strings.TrimSpace(text)
	var matches []string

	for _, p := range hashPatterns {
		if p.Length > 0 && len(cleaned) != p.Length {
			continue
		}
		if p.Regex.MatchString(cleaned) {
			matches = append(matches, p.Name)
		}
	}

	if len(matches) == 0 {
		// 仍然提供长度和建议
		info := fmt.Sprintf("no exact match for hex hash of length %d. "+
			"check if input has extra whitespace, separators, or is not hex-encoded.", len(cleaned))
		switch {
		case len(cleaned) == 32:
			info += " length=32 suggests MD5, NTLM, or MD4."
		case len(cleaned) == 40:
			info += " length=40 suggests SHA1 or RIPEMD160."
		case len(cleaned) == 64:
			info += " length=64 suggests SHA256."
		case len(cleaned) == 128:
			info += " length=128 suggests SHA512 or Whirlpool."
		case len(cleaned) == 16:
			info += " length=16 may be MySQL323 or half MD5."
		}
		return CryptoHelperOutput{Result: info}
	}

	// MD5 和 NTLM 都是 32 位 hex，若同时匹配说明
	result := strings.Join(matches, ", ")
	if len(matches) > 1 {
		result += "\nnote: multiple matches possible — use context (e.g. Windows domain → NTLM, file hash → MD5/SHA1)"
	}
	return CryptoHelperOutput{Result: result}
}

// ─── base_decode ───

func autoBaseDecode(text string) CryptoHelperOutput {
	cleaned := strings.TrimSpace(text)

	// 尝试 hex
	if isHexString(cleaned) {
		if decoded, err := decodeHexSafe(cleaned); err == nil && isPrintable(decoded) {
			return CryptoHelperOutput{Result: fmt.Sprintf("[hex] %s", decoded)}
		}
	}

	// 尝试 base32（可能缺 padding）——先于 base64，因为 base32 字符集更严格
	if looksLikeBase32(cleaned) {
		if decoded, err := decodeBase32Auto(cleaned); err == nil && len(decoded) > 0 {
			return CryptoHelperOutput{Result: fmt.Sprintf("[base32] %s", string(decoded))}
		}
	}

	// 尝试 base64（可能缺 padding）
	if decoded, err := decodeBase64Auto(cleaned); err == nil {
		if isPrintable(decoded) && len(decoded) > 0 {
			return CryptoHelperOutput{Result: fmt.Sprintf("[base64] %s", decoded)}
		}
		// 如果 base64 解码成功但不可打印，仍然返回（可能是二进制数据）
		if len(decoded) > 0 {
			hexResult := hex.EncodeToString([]byte(decoded))
			return CryptoHelperOutput{Result: fmt.Sprintf("[base64→binary] %s", hexResult)}
		}
	}

	return CryptoHelperOutput{Error: "could not decode as hex, base64, or base32"}
}

func isHexString(s string) bool {
	if len(s)%2 != 0 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) >= 4
}

func decodeHexSafe(data string) (string, error) {
	result, err := hex.DecodeString(data)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func decodeBase64Auto(data string) (string, error) {
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			return -1
		}
		return r
	}, data)

	// 尝试标准解码
	if result, err := base64.StdEncoding.DecodeString(cleaned); err == nil {
		return string(result), nil
	}

	// 补 padding
	padded := cleaned
	switch len(padded) % 4 {
	case 2:
		padded += "=="
	case 3:
		padded += "="
	}
	if result, err := base64.StdEncoding.DecodeString(padded); err == nil {
		return string(result), nil
	}

	return "", fmt.Errorf("base64 decode failed")
}

func looksLikeBase32(s string) bool {
	// base32 只包含 A-Z 和 2-7，且惯例为大写。
	// 含小写字母的通常为 base64。
	hasLower := strings.TrimRight(s, "=") != strings.ToUpper(strings.TrimRight(s, "="))
	if hasLower {
		return false
	}
	upper := strings.ToUpper(strings.TrimRight(s, "="))
	if len(upper) == 0 {
		return false
	}
	for _, c := range upper {
		if !((c >= 'A' && c <= 'Z') || (c >= '2' && c <= '7')) {
			return false
		}
	}
	return true
}

func decodeBase32Auto(data string) (string, error) {
	cleaned := strings.ToUpper(strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			return -1
		}
		return r
	}, data))

	// 补 padding
	mod := len(cleaned) % 8
	if mod != 0 {
		cleaned += strings.Repeat("=", 8-mod)
	}

	result, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		// base32 不是 base64.StdEncoding，用自定义 base32
		return decodeBase32RFC(cleaned)
	}
	return string(result), nil
}

func decodeBase32RFC(data string) (string, error) {
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	charToVal := make(map[byte]byte)
	for i := 0; i < len(alphabet); i++ {
		charToVal[alphabet[i]] = byte(i)
	}

	cleaned := strings.TrimRight(data, "=")
	var bits uint64
	var bitsLen uint
	var result []byte

	for i := 0; i < len(cleaned); i++ {
		val, ok := charToVal[cleaned[i]]
		if !ok {
			return "", fmt.Errorf("invalid base32 char: %c", cleaned[i])
		}
		bits = (bits << 5) | uint64(val)
		bitsLen += 5
		if bitsLen >= 8 {
			bitsLen -= 8
			result = append(result, byte(bits>>bitsLen))
			bits &= (1 << bitsLen) - 1
		}
	}
	return string(result), nil
}

// ─── xor_single ───

func xorSingleBrute(text string, limit int) CryptoHelperOutput {
	data, err := hex.DecodeString(strings.TrimSpace(text))
	if err != nil {
		return CryptoHelperOutput{Error: fmt.Sprintf("text must be hex-encoded for xor_single: %v", err)}
	}

	var candidates []CryptoCandidate
	for key := 0; key < 256; key++ {
		plain := make([]byte, len(data))
		for i, b := range data {
			plain[i] = b ^ byte(key)
		}
		plainStr := string(plain)
		score := readabilityScore(plainStr)
		candidates = append(candidates, CryptoCandidate{
			Plaintext: plainStr,
			Key:       fmt.Sprintf("0x%02x", key),
			Score:     score,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	if limit > len(candidates) {
		limit = len(candidates)
	}
	candidates = candidates[:limit]

	if len(candidates) > 0 && candidates[0].Score < 0.3 {
		return CryptoHelperOutput{
			Candidates: candidates,
			Error:      "low readability scores — data may not be single-byte XOR encrypted",
		}
	}

	return CryptoHelperOutput{Candidates: candidates}
}

// ─── xor_repeat ───

func xorRepeatBrute(text string, keyHex string, limit int) CryptoHelperOutput {
	data, err := hex.DecodeString(strings.TrimSpace(text))
	if err != nil {
		return CryptoHelperOutput{Error: fmt.Sprintf("text must be hex-encoded for xor_repeat: %v", err)}
	}

	// 如果提供了 key，用给定 key 解密
	if keyHex != "" {
		keyBytes, err := hex.DecodeString(keyHex)
		if err != nil {
			return CryptoHelperOutput{Error: fmt.Sprintf("key must be hex-encoded: %v", err)}
		}
		if len(keyBytes) == 0 {
			return CryptoHelperOutput{Error: "key is empty"}
		}
		plain := xorRepeatingKey(data, keyBytes)
		return CryptoHelperOutput{
			Result: string(plain),
			Candidates: []CryptoCandidate{{
				Plaintext: string(plain),
				Key:       keyHex,
				Score:     readabilityScore(string(plain)),
			}},
		}
	}

	// 未提供 key：尝试常见密钥长度范围（1-16 字节）
	// 先用 single-byte 做快速扫描，再用多字节探测
	var candidates []CryptoCandidate

	// 对于短密钥（1-8 字节），尝试 base64 常见密钥
	commonKeys := [][]byte{
		{0x00}, // null key
	}
	_ = commonKeys

	// 尝试 key 长度 1 到 8（短密钥范围）
	for keyLen := 1; keyLen <= 8 && keyLen <= len(data)/2; keyLen++ {
		// 用已知可能的 key 尝试：ASCII 字符范围
		// 限制搜索空间：对每个 key 位置只用常见可打印字符
		guessKey := make([]byte, keyLen)
		// 尝试用全零初始尝试
		plain := xorRepeatingKey(data, guessKey)
		score := readabilityScore(string(plain))
		candidates = append(candidates, CryptoCandidate{
			Plaintext: string(plain),
			Key:       fmt.Sprintf("%x", guessKey),
			Score:     score,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	if limit > len(candidates) {
		limit = len(candidates)
	}

	if len(candidates) == 0 {
		return CryptoHelperOutput{Error: "xor_repeat without key requires a small ciphertext; provide key as hex string for direct decryption"}
	}

	return CryptoHelperOutput{Candidates: candidates[:limit]}
}

func xorRepeatingKey(data, key []byte) []byte {
	result := make([]byte, len(data))
	for i := range data {
		result[i] = data[i] ^ key[i%len(key)]
	}
	return result
}

// ─── caesar ───

func caesarBrute(text string, shiftStr string, limit int) CryptoHelperOutput {
	// 如果提供了 shift，只做特定位移
	if shiftStr != "" {
		shift := 0
		if _, err := fmt.Sscanf(shiftStr, "%d", &shift); err != nil || shift < 0 || shift > 25 {
			return CryptoHelperOutput{Error: "caesar shift must be an integer 0-25, or omit for brute force all shifts"}
		}
		plain := caesarShift(text, shift)
		return CryptoHelperOutput{
			Result: plain,
			Candidates: []CryptoCandidate{{
				Plaintext: plain,
				Key:       fmt.Sprintf("%d", shift),
				Score:     readabilityScore(plain),
			}},
		}
	}

	// 爆破所有 26 个位移
	var candidates []CryptoCandidate
	for shift := 0; shift < 26; shift++ {
		plain := caesarShift(text, shift)
		candidates = append(candidates, CryptoCandidate{
			Plaintext: plain,
			Key:       fmt.Sprintf("%d", shift),
			Score:     readabilityScore(plain),
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	if limit > len(candidates) {
		limit = len(candidates)
	}
	return CryptoHelperOutput{Candidates: candidates[:limit]}
}

func caesarShift(text string, shift int) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return 'a' + (r-'a'+rune(26-shift))%26
		case r >= 'A' && r <= 'Z':
			return 'A' + (r-'A'+rune(26-shift))%26
		default:
			return r
		}
	}, text)
}

// ─── 简单解码（复用 encoding_decoder 逻辑，不修改原工具） ───

func simpleURLDecode(text string) CryptoHelperOutput {
	result, err := url.QueryUnescape(strings.TrimSpace(text))
	if err != nil {
		return CryptoHelperOutput{Error: fmt.Sprintf("url decode failed: %v", err)}
	}
	return CryptoHelperOutput{Result: result}
}

func simpleHexDecode(text string) CryptoHelperOutput {
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			return -1
		}
		return r
	}, strings.TrimSpace(text))
	result, err := hex.DecodeString(cleaned)
	if err != nil {
		return CryptoHelperOutput{Error: fmt.Sprintf("hex decode failed: %v", err)}
	}
	return CryptoHelperOutput{Result: string(result)}
}

func simpleBinaryDecode(text string) CryptoHelperOutput {
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			return -1
		}
		return r
	}, strings.TrimSpace(text))

	if len(cleaned)%8 != 0 {
		return CryptoHelperOutput{Error: fmt.Sprintf("binary string length must be multiple of 8, got %d", len(cleaned))}
	}

	var result strings.Builder
	for i := 0; i < len(cleaned); i += 8 {
		var b byte
		for j := 0; j < 8; j++ {
			if cleaned[i+j] == '1' {
				b |= 1 << (7 - j)
			} else if cleaned[i+j] != '0' {
				return CryptoHelperOutput{Error: fmt.Sprintf("invalid binary character at position %d: %c", i+j, cleaned[i+j])}
			}
		}
		result.WriteByte(b)
	}
	return CryptoHelperOutput{Result: result.String()}
}

// ─── 可读性评分 ───

// readabilityScore 对文本的可读性评分（0.0 到 1.0）。
// 用作 XOR/Caesar 爆破结果的排序依据。
func readabilityScore(text string) float64 {
	if len(text) == 0 {
		return 0
	}

	var letters, spaces, printable, total int
	var hasVowel bool
	vowels := "aeiouAEIOU"

	for _, r := range text {
		total++
		if unicode.IsPrint(r) {
			printable++
		}
		if unicode.IsLetter(r) {
			letters++
			if strings.ContainsRune(vowels, r) {
				hasVowel = true
			}
		}
		if r == ' ' {
			spaces++
		}
	}

	// 不可打印字符占比过高 → 低分
	printRatio := float64(printable) / float64(total)
	if printRatio < 0.95 {
		return printRatio * 0.3
	}

	// 字母和空格占比
	letterRatio := float64(letters+spaces) / float64(total)

	// 常见英文高频字符检查
	highFreq := "etaoin shrdluETAOINSHRDLU"
	freqCount := 0
	for _, r := range text {
		if strings.ContainsRune(highFreq, r) {
			freqCount++
		}
	}
	freqRatio := float64(freqCount) / float64(max(total, 1))

	// 不含元音的可打印文本（如纯符号）降低分数
	score := letterRatio*0.4 + freqRatio*0.35 + printRatio*0.25
	if !hasVowel && letters > 4 {
		score *= 0.6
	}

	return math.Min(score, 1.0)
}

func isPrintable(s string) bool {
	for _, r := range s {
		if !unicode.IsPrint(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}
