package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/oskar1233/ksef/internal/settings"
)

func Download(month string, dir string) error {
	cfg, client, err := loadSettingsAndClient()
	if err != nil {
		return err
	}
	if month == "" {
		month = currentMonth()
	}
	if dir == "" {
		dir = cfg.DownloadDir
	}

	invoices, err := queryInvoicesForMonth(cfg, client, month)
	if err != nil {
		return err
	}

	downloaded := 0
	for _, invoice := range invoices {
		path := invoiceFilePath(dir, month, invoice)
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("skip existing %s\n", path)
			continue
		}

		accessToken, err := ensureAccessToken(cfg, client)
		if err != nil {
			return err
		}

		content, _, err := client.DownloadInvoice(accessToken.Token, invoice.KSeFNumber)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create directory for %s: %w", path, err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}

		cfg.LastInvoiceDownload = &settings.InvoiceDownloadState{
			Month:        month,
			Directory:    dir,
			KSeFNumber:   invoice.KSeFNumber,
			File:         path,
			DownloadedAt: time.Now().Format(time.RFC3339Nano),
		}
		if err := settings.Save(cfg); err != nil {
			return err
		}

		fmt.Printf("downloaded %s\n", path)
		downloaded++
	}

	fmt.Printf("downloaded %d invoices\n", downloaded)
	return nil
}
