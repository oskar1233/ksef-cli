package commands

import (
	"fmt"
	"os"

	"github.com/oskar1233/ksef/internal/render"
)

func DownloadPDFs(month string, dir string, subject string, force bool, keepHTML bool) error {
	cfg, client, err := loadSettingsAndClient()
	if err != nil {
		return err
	}
	if month == "" {
		month = currentMonth()
	}
	if dir == "" {
		dir = cfg.PDFDir
	}

	grouped, err := queryInvoicesForKinds(cfg, client, month, subject, invoiceKindBoth)
	if err != nil {
		return err
	}

	totalDownloaded := 0
	for _, kind := range []string{invoiceKindPurchase, invoiceKindSales} {
		invoices, exists := grouped[kind]
		if !exists {
			continue
		}

		for _, invoice := range invoices {
			path := invoicePDFPath(dir, month, kind, invoice)
			htmlPath := ""
			if keepHTML {
				htmlPath = invoiceHTMLPath(dir, month, kind, invoice)
			}
			if !force {
				if _, err := os.Stat(path); err == nil {
					fmt.Printf("skip existing %s\n", path)
					continue
				}
			}

			accessToken, err := ensureAccessToken(cfg, client)
			if err != nil {
				return err
			}

			xmlContent, _, err := client.DownloadInvoice(accessToken.Token, invoice.KSeFNumber)
			if err != nil {
				return err
			}

			if err := render.RenderInvoicePDFWithOptions(invoice, xmlContent, kind, path, render.PDFRenderOptions{HTMLPath: htmlPath}); err != nil {
				return err
			}

			fmt.Printf("rendered %s\n", path)
			if htmlPath != "" {
				fmt.Printf("saved %s\n", htmlPath)
			}
			totalDownloaded++
		}
	}

	fmt.Printf("rendered %d PDFs\n", totalDownloaded)
	return nil
}

func DownloadLastMonthPDFs(dir string, subject string, force bool, keepHTML bool) error {
	return DownloadPDFs(lastMonth(), dir, subject, force, keepHTML)
}
