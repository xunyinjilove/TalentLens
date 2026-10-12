package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// DOCX is an OOXML ZIP archive, not a plain text file. Read only the main body;
// preserve paragraph/table-cell boundaries and never send archive bytes to AI.
func extractDOCXText(filePath string) (string, error) {
	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return "", err
	}
	defer archive.Close()
	const maxXMLSize = 16 << 20
	for _, entry := range archive.File {
		if entry.Name != "word/document.xml" {
			continue
		}
		if entry.UncompressedSize64 > maxXMLSize {
			return "", fmt.Errorf("DOCX body exceeds size limit")
		}
		r, err := entry.Open()
		if err != nil {
			return "", err
		}
		data, readErr := io.ReadAll(io.LimitReader(r, maxXMLSize+1))
		r.Close()
		if readErr != nil {
			return "", readErr
		}
		if len(data) > maxXMLSize {
			return "", fmt.Errorf("DOCX body exceeds size limit")
		}
		decoder := xml.NewDecoder(bytes.NewReader(data))
		var text strings.Builder
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", err
			}
			switch token := token.(type) {
			case xml.StartElement:
				switch token.Name.Local {
				case "t":
					var run string
					if err := decoder.DecodeElement(&run, &token); err != nil {
						return "", err
					}
					text.WriteString(run)
				case "tab":
					text.WriteByte('\t')
				case "br", "cr":
					text.WriteByte('\n')
				}
			case xml.EndElement:
				if token.Name.Local == "p" {
					text.WriteByte('\n')
				}
			}
		}
		return strings.TrimSpace(text.String()), nil
	}
	return "", fmt.Errorf("DOCX body is missing")
}
