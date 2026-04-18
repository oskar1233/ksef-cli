package commands

func ListLastMonth(output string, subject string) error {
	cfg, client, err := loadSettingsAndClient()
	if err != nil {
		return err
	}
	if output == "" {
		output = "table"
	}

	grouped, err := queryInvoicesForKinds(cfg, client, lastMonth(), subject, invoiceKindBoth)
	if err != nil {
		return err
	}

	return printInvoicesGrouped(grouped, output)
}
