package tool

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// EncodingDecoderInput 编码解码工具的输入参数。
type EncodingDecoderInput struct {
	Data     string `json:"data" jsonschema:"description=encoded data to decode"`
	Encoding string `json:"encoding" jsonschema:"description=encoding type: hex, base64, url, rot13, binary"`
}

// EncodingDecoderOutput 编码解码工具的输出结果。
type EncodingDecoderOutput struct {
	Decoded   string `json:"decoded" jsonschema:"description=decoded result, truncated if exceeds limit"`
	Truncated bool   `json:"truncated" jsonschema:"description=whether the result was truncated"`
	Error     string `json:"error,omitempty" jsonschema:"description=error message if decoding fails"`
}

// NewEncodingDecoderTool 创建编码解码工具，支持 hex、base64、url、rot13、binary 解码。
func NewEncodingDecoderTool() (einotool.InvokableTool, error) {
	return utils.InferTool[EncodingDecoderInput, EncodingDecoderOutput](
		"encoding_decoder",
		"Decode common encoding formats found in CTF challenges. "+
			"Supports hex (hexadecimal), base64, url (URL encoding), rot13 (ROT13 cipher), "+
			"and binary (binary string to text). "+
			"Use this when you encounter encoded strings in challenge files or prompts.",
		func(ctx context.Context, input EncodingDecoderInput) (EncodingDecoderOutput, error) {
			encoding := strings.ToLower(strings.TrimSpace(input.Encoding))
			data := strings.TrimSpace(input.Data)
			if data == "" {
				return EncodingDecoderOutput{Error: "data is empty"}, nil
			}

			decoded, err := decodeData(data, encoding)
			if err != nil {
				return EncodingDecoderOutput{Error: err.Error()}, nil
			}

			result, truncated := truncateOutput(decoded, defaultOutputLimit)
			if truncated {
				result += "\n...[decoded output truncated]"
			}
			return EncodingDecoderOutput{
				Decoded:   result,
				Truncated: truncated,
			}, nil
		},
	)
}

func decodeData(data, encoding string) (string, error) {
	switch encoding {
	case "hex":
		return decodeHex(data)
	case "base64":
		return decodeBase64(data)
	case "url":
		return decodeURL(data)
	case "rot13":
		return decodeROT13(data), nil
	case "binary":
		return decodeBinary(data)
	default:
		return "", fmt.Errorf("unsupported encoding: %s (supported: hex, base64, url, rot13, binary)", encoding)
	}
}

func decodeHex(data string) (string, error) {
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			return -1
		}
		return r
	}, data)
	result, err := hex.DecodeString(cleaned)
	if err != nil {
		return "", fmt.Errorf("hex decode failed: %w", err)
	}
	return string(result), nil
}

func decodeBase64(data string) (string, error) {
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			return -1
		}
		return r
	}, data)
	result, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}
	return string(result), nil
}

func decodeURL(data string) (string, error) {
	result, err := url.QueryUnescape(data)
	if err != nil {
		return "", fmt.Errorf("url decode failed: %w", err)
	}
	return result, nil
}

func decodeROT13(data string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return 'a' + (r-'a'+13)%26
		case r >= 'A' && r <= 'Z':
			return 'A' + (r-'A'+13)%26
		default:
			return r
		}
	}, data)
}

func decodeBinary(data string) (string, error) {
	cleaned := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			return -1
		}
		return r
	}, data)

	if len(cleaned)%8 != 0 {
		return "", fmt.Errorf("binary string length must be multiple of 8, got %d", len(cleaned))
	}

	var result strings.Builder
	for i := 0; i < len(cleaned); i += 8 {
		var b byte
		for j := 0; j < 8; j++ {
			if cleaned[i+j] == '1' {
				b |= 1 << (7 - j)
			} else if cleaned[i+j] != '0' {
				return "", fmt.Errorf("invalid binary character at position %d: %c", i+j, cleaned[i+j])
			}
		}
		result.WriteByte(b)
	}
	return result.String(), nil
}
