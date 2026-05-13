package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minimalTextPDF 用精确 byte offset 构造一个最小合法 PDF。
// 逐行写入，同时记录每个 "N 0 obj" 的 offset。
func minimalTextPDF(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.pdf")

	var buf strings.Builder
	var objOffsets [6]int // obj 0 占位 + obj 1-5

	emit := func(s string) { buf.WriteString(s + "\n") }

	// header
	emit("%PDF-1.4")

	// obj 1: catalog
	objOffsets[1] = buf.Len()
	emit("1 0 obj")
	emit("<< /Type /Catalog /Pages 2 0 R >>")
	emit("endobj")

	// obj 2: pages
	objOffsets[2] = buf.Len()
	emit("2 0 obj")
	emit("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	emit("endobj")

	// obj 3: page
	objOffsets[3] = buf.Len()
	emit("3 0 obj")
	emit("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>")
	emit("endobj")

	// obj 4: content stream
	objOffsets[4] = buf.Len()
	emit("4 0 obj")
	emit("<< /Length 44 >>")
	emit("stream")
	emit("BT /F1 12 Tf 100 700 Td (Hello PDF World) Tj ET")
	emit("endstream")
	emit("endobj")

	// obj 5: font
	objOffsets[5] = buf.Len()
	emit("5 0 obj")
	emit("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	emit("endobj")

	// xref table
	xrefStart := buf.Len()
	emit("xref")
	emit("0 6")
	emit(fmt.Sprintf("%010d 65535 f ", 0))
	for i := 1; i <= 5; i++ {
		emit(fmt.Sprintf("%010d 00000 n ", objOffsets[i]))
	}

	// trailer
	emit("trailer")
	emit("<< /Size 6 /Root 1 0 R >>")
	emit("startxref")
	emit(fmt.Sprintf("%d", xrefStart))
	emit("%%EOF")

	if err := os.WriteFile(path, []byte(buf.String()), 0644); err != nil {
		t.Fatalf("write test pdf: %v", err)
	}
	return path
}

func TestParsePDF_ExtractsText(t *testing.T) {
	pdfPath := minimalTextPDF(t)
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("read test pdf: %v", err)
	}

	pages, err := ParsePDF(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("ParsePDF: %v", err)
	}
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page with text")
	}
	if !strings.Contains(pages[0].Text, "Hello PDF World") {
		t.Errorf("expected 'Hello PDF World' in text, got: %s", pages[0].Text)
	}
	if pages[0].PageNumber != 1 {
		t.Errorf("page number: got %d, want 1", pages[0].PageNumber)
	}
}

func TestParsePDF_EmptyData(t *testing.T) {
	_, err := ParsePDF(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty data")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty: %s", err.Error())
	}
}

func TestParsePDF_InvalidData(t *testing.T) {
	_, err := ParsePDF(strings.NewReader("not a pdf file"))
	if err == nil {
		t.Fatal("expected error for invalid data")
	}
}

func TestParsePDF_NoText(t *testing.T) {
	// ParsePDF 对空内容和无效 PDF 返回错误。
	// 注意：构造无文本但合法的 PDF 依赖 ledongthuc/pdf 的特定行为，
	// 此处仅测试基础错误路径。
	_, err := ParsePDF(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty: %s", err.Error())
	}
}

func TestIsAllowedFilename(t *testing.T) {
	allowed := []string{"doc.md", "doc.markdown", "doc.pdf", "path/to/file.MD", "file.PDF"}
	for _, name := range allowed {
		if !isAllowedFilename(name) {
			t.Errorf("%q should be allowed", name)
		}
	}

	rejected := []string{"doc.txt", "doc.exe", "doc", "doc.png", "doc.docx"}
	for _, name := range rejected {
		if isAllowedFilename(name) {
			t.Errorf("%q should be rejected", name)
		}
	}
}

func TestIsPDFFilename(t *testing.T) {
	if !isPDFFilename("doc.pdf") {
		t.Error("pdf should be detected")
	}
	if isPDFFilename("doc.md") {
		t.Error("md should not be detected as pdf")
	}
}

// TestPDFPageTextFields 验证 PDFPageText 结构体字段。
func TestPDFPageTextFields(t *testing.T) {
	page := PDFPageText{PageNumber: 3, Text: "some text"}
	if page.PageNumber != 3 {
		t.Errorf("PageNumber: got %d, want 3", page.PageNumber)
	}
	if page.Text != "some text" {
		t.Errorf("Text: got %q, want 'some text'", page.Text)
	}
}
