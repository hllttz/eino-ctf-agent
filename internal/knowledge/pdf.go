package knowledge

import (
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFPageText 单页 PDF 文本提取结果。
type PDFPageText struct {
	PageNumber int
	Text       string
}

// ParsePDF 从 reader 读取 PDF，按页提取纯文本。
// 空页跳过；整个 PDF 无可提取文本时返回明确错误。
// 不支持 OCR、不处理加密 PDF。
func ParsePDF(reader io.Reader) ([]PDFPageText, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read pdf: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("pdf file is empty")
	}

	pdfReader, err := pdf.NewReader(strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}

	totalPages := pdfReader.NumPage()
	if totalPages == 0 {
		return nil, fmt.Errorf("pdf has no pages")
	}

	var pages []PDFPageText
	for i := 1; i <= totalPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		pages = append(pages, PDFPageText{
			PageNumber: i,
			Text:       text,
		})
	}

	if len(pages) == 0 {
		return nil, fmt.Errorf("pdf has no extractable text; OCR is not supported")
	}
	return pages, nil
}
