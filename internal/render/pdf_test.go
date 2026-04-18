package render

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	ksef "github.com/oskar1233/ksef/internal"
	"github.com/stretchr/testify/require"
)

func TestExtractXMLFields(t *testing.T) {
	xmlContent := []byte(`<?xml version="1.0" encoding="utf-8"?>
<Invoice>
  <Header number="FV/1/04/2026">
    <IssueDate>2026-04-02</IssueDate>
  </Header>
  <Seller>
    <Name>Demo Seller</Name>
  </Seller>
  <Items>
    <Item><Name>Service A</Name><Gross>123.45</Gross></Item>
    <Item><Name>Service B</Name><Gross>50.00</Gross></Item>
  </Items>
</Invoice>`)

	fields, err := ExtractXMLFields(xmlContent)
	require.NoError(t, err)
	require.NotEmpty(t, fields)
	require.Contains(t, fields, XMLField{Path: "Invoice/Header@number", Value: "FV/1/04/2026"})
	require.Contains(t, fields, XMLField{Path: "Invoice/Header/IssueDate", Value: "2026-04-02"})
	require.Contains(t, fields, XMLField{Path: "Invoice/Items/Item[1]/Name", Value: "Service A"})
	require.Contains(t, fields, XMLField{Path: "Invoice/Items/Item[2]/Gross", Value: "50.00"})
}

func TestRenderInvoiceHTML(t *testing.T) {
	htmlContent, err := RenderInvoiceHTML(sampleInvoiceMetadata(), sampleInvoiceXML(), "purchase")
	require.NoError(t, err)
	html := string(htmlContent)
	require.Contains(t, html, "<!doctype html>")
	require.Contains(t, html, "Zakup")
	require.Contains(t, html, "Krajowy System e-Faktur")
	require.Contains(t, html, "Nazwa towaru lub usługi")
	require.Contains(t, html, "Consulting service with a very long label that should wrap cleanly in HTML")
}

func TestRenderInvoicePDF(t *testing.T) {
	restore := SetPDFGeneratorForTesting(func(ctx context.Context, htmlPath string, outputPath string) error {
		content, err := os.ReadFile(htmlPath)
		require.NoError(t, err)
		require.Contains(t, string(content), "FV/1/04/2026")
		return os.WriteFile(outputPath, []byte("%PDF-1.4\n%dummy\n"), 0o644)
	})
	defer restore()

	outputPath := filepath.Join(t.TempDir(), "invoice.pdf")
	err := RenderInvoicePDF(sampleInvoiceMetadata(), sampleInvoiceXML(), "purchase", outputPath)
	require.NoError(t, err)

	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	require.True(t, info.Size() > 0)
}

func TestRenderInvoicePDFWithOptionsWritesIntermediateHTML(t *testing.T) {
	restore := SetPDFGeneratorForTesting(func(ctx context.Context, htmlPath string, outputPath string) error {
		return os.WriteFile(outputPath, []byte("%PDF-1.4\n%dummy\n"), 0o644)
	})
	defer restore()

	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "invoice.pdf")
	htmlPath := filepath.Join(tempDir, "invoice.html")

	err := RenderInvoicePDFWithOptions(sampleInvoiceMetadata(), sampleInvoiceXML(), "purchase", outputPath, PDFRenderOptions{HTMLPath: htmlPath})
	require.NoError(t, err)

	htmlContent, err := os.ReadFile(htmlPath)
	require.NoError(t, err)
	require.Contains(t, string(htmlContent), "FV/1/04/2026")
	require.Contains(t, string(htmlContent), "Zakup")
}

func sampleInvoiceMetadata() ksef.InvoiceMetadata {
	return ksef.InvoiceMetadata{
		KSeFNumber:    "1234567890-202604-01",
		InvoiceNumber: "FV/1/04/2026",
		IssueDate:     "2026-04-02",
		Seller:        ksef.InvoiceMetadataSeller{Name: "Demo Seller", NIP: "1234567890"},
		Buyer:         ksef.InvoiceMetadataBuyer{Name: "Demo Buyer", Identifier: ksef.AuthenticationContextIdentifier{Type: "Nip", Value: "0987654321"}},
		NetAmount:     100,
		GrossAmount:   123.45,
		VATAmount:     23.45,
		Currency:      "PLN",
	}
}

func sampleInvoiceXML() []byte {
	return []byte(`<?xml version="1.0" encoding="utf-8"?>
<Faktura>
  <Podmiot1>
    <DaneIdentyfikacyjne>
      <NIP>1234567890</NIP>
      <Nazwa>Demo Seller</Nazwa>
    </DaneIdentyfikacyjne>
    <Adres>
      <AdresL1>Main Street 1</AdresL1>
      <AdresL2>00-001 Warsaw</AdresL2>
      <KodKraju>PL</KodKraju>
    </Adres>
  </Podmiot1>
  <Podmiot2>
    <DaneIdentyfikacyjne>
      <NIP>0987654321</NIP>
      <Nazwa>Demo Buyer</Nazwa>
    </DaneIdentyfikacyjne>
  </Podmiot2>
  <Fa>
    <P_1>2026-04-02</P_1>
    <P_2>FV/1/04/2026</P_2>
    <KodWaluty>PLN</KodWaluty>
    <RodzajFaktury>VAT</RodzajFaktury>
    <P_15>123.45</P_15>
    <FaWiersz>
      <NrWierszaFa>1</NrWierszaFa>
      <P_7>Consulting service with a very long label that should wrap cleanly in HTML</P_7>
      <P_8A>h</P_8A>
      <P_8B>10</P_8B>
      <P_9A>10.00</P_9A>
      <P_11>100.00</P_11>
      <P_12>23</P_12>
    </FaWiersz>
    <P_13_1>100.00</P_13_1>
    <P_14_1>23.00</P_14_1>
    <Platnosc>
      <Zaplacono>2</Zaplacono>
      <TerminPlatnosci>
        <Termin>2026-04-10</Termin>
      </TerminPlatnosci>
      <FormaPlatnosci>6</FormaPlatnosci>
    </Platnosc>
  </Fa>
</Faktura>`)
}
