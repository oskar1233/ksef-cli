package render

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"

	ksef "github.com/oskar1233/ksef/internal"
)

var (
	//go:embed templates/invoice.html.tmpl
	invoiceHTMLTemplate string

	//go:embed templates/invoice.css
	invoiceCSS string
)

type invoiceHTMLView struct {
	CSS      template.CSS
	Document invoiceDocument
}

func RenderInvoiceHTML(invoice ksef.InvoiceMetadata, xmlContent []byte, subjectLabel string) ([]byte, error) {
	root, err := parseXML(xmlContent)
	if err != nil {
		return nil, fmt.Errorf("parse invoice xml: %w", err)
	}

	document := buildInvoiceDocument(root, invoice, subjectLabel)
	return renderInvoiceHTMLDocument(document)
}

func renderInvoiceHTMLDocument(document invoiceDocument) ([]byte, error) {
	tmpl, err := template.New("invoice").Parse(invoiceHTMLTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse invoice html template: %w", err)
	}

	view := invoiceHTMLView{
		CSS:      template.CSS(invoiceCSS),
		Document: document,
	}

	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, view); err != nil {
		return nil, fmt.Errorf("render invoice html template: %w", err)
	}
	return buffer.Bytes(), nil
}
