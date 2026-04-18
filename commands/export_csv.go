package commands

import "fmt"

func ExportCSV(month string, dir string, subject string) error {
	cfg, client, err := loadSettingsAndClient()
	if err != nil {
		return err
	}
	if month == "" {
		month = currentMonth()
	}
	if dir == "" {
		dir = cfg.ExportDir
	}

	grouped, err := queryInvoicesForKinds(cfg, client, month, subject, invoiceKindBoth)
	if err != nil {
		return err
	}

	for _, kind := range []string{invoiceKindPurchase, invoiceKindSales} {
		invoices, exists := grouped[kind]
		if !exists {
			continue
		}
		path := fmt.Sprintf("%s/%s_%s.csv", dir, kind, month)
		if err := writeInvoicesCSVFile(path, kind, invoices); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", path)
	}

	return nil
}
