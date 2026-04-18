package commands

func TokenAuth() error {
	cfg, client, err := loadSettingsAndClient()
	if err != nil {
		return err
	}
	return tokenAuthFlow(cfg, client)
}
