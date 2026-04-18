package commands

func ListInvoices(month string, output string) error {
	cfg, client, err := loadSettingsAndClient()
	if err != nil {
		return err
	}
	if month == "" {
		month = currentMonth()
	}
	if output == "" {
		output = "table"
	}

	invoices, err := queryInvoicesForMonth(cfg, client, month)
	if err != nil {
		return err
	}

	return printInvoices(invoices, output)
}
