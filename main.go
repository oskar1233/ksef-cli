package main

import (
	"fmt"
	"os"
	"time"

	"github.com/oskar1233/ksef/commands"
	"github.com/oskar1233/ksef/internal/settings"
	"github.com/spf13/cobra"
)

func main() {
	defaults := settings.DefaultSettings()

	rootCmd := &cobra.Command{
		Use:          "ksef",
		Short:        "KSeF CLI",
		SilenceUsage: true,
	}

	rootCmd.AddCommand(initCommand(defaults))
	rootCmd.AddCommand(challengeCommand())
	rootCmd.AddCommand(authorizeCommand())
	rootCmd.AddCommand(authStatusCommand())
	rootCmd.AddCommand(redeemCommand())
	rootCmd.AddCommand(refreshCommand())
	rootCmd.AddCommand(generateTokenCommand())
	rootCmd.AddCommand(tokenAuthCommand())
	rootCmd.AddCommand(listInvoicesCommand())
	rootCmd.AddCommand(listLastMonthCommand())
	rootCmd.AddCommand(downloadCommand())
	rootCmd.AddCommand(downloadPDFsCommand())
	rootCmd.AddCommand(downloadLastMonthPDFsCommand())
	rootCmd.AddCommand(exportCSVCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func initCommand(defaults *settings.Settings) *cobra.Command {
	var nip string
	var environment string
	var baseURL string
	var subjectIdentifierType string
	var verifyCertificateChain bool
	var authRequestFile string
	var signedAuthRequestFile string
	var downloadDir string
	var pdfDir string
	var exportDir string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize or update ~/.ksef/settings.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			options := commands.InitOptions{}
			if cmd.Flags().Changed("nip") {
				options.NIP = &nip
			}
			if cmd.Flags().Changed("environment") {
				options.Environment = &environment
			}
			if cmd.Flags().Changed("base-url") {
				options.BaseURL = &baseURL
			}
			if cmd.Flags().Changed("subject-identifier-type") {
				options.SubjectIdentifierType = &subjectIdentifierType
			}
			if cmd.Flags().Changed("verify-certificate-chain") {
				options.VerifyCertificateChain = &verifyCertificateChain
			}
			if cmd.Flags().Changed("auth-request-file") {
				options.AuthRequestFile = &authRequestFile
			}
			if cmd.Flags().Changed("signed-auth-request-file") {
				options.SignedAuthRequestFile = &signedAuthRequestFile
			}
			if cmd.Flags().Changed("download-dir") {
				options.DownloadDir = &downloadDir
			}
			if cmd.Flags().Changed("pdf-dir") {
				options.PDFDir = &pdfDir
			}
			if cmd.Flags().Changed("export-dir") {
				options.ExportDir = &exportDir
			}
			return commands.Init(options)
		},
	}

	cmd.Flags().StringVar(&nip, "nip", "", "Context NIP")
	cmd.Flags().StringVar(&environment, "environment", defaults.Environment, "Environment: demo, test, production")
	cmd.Flags().StringVar(&baseURL, "base-url", defaults.BaseURL, "Custom API base URL")
	cmd.Flags().StringVar(&subjectIdentifierType, "subject-identifier-type", defaults.SubjectIdentifierType, "certificateSubject or certificateFingerprint")
	cmd.Flags().BoolVar(&verifyCertificateChain, "verify-certificate-chain", defaults.VerifyCertificateChain, "Verify certificate chain on /auth/xades-signature")
	cmd.Flags().StringVar(&authRequestFile, "auth-request-file", defaults.AuthRequestFile, "Default output path for unsigned auth request XML")
	cmd.Flags().StringVar(&signedAuthRequestFile, "signed-auth-request-file", defaults.SignedAuthRequestFile, "Default input path for signed auth request XML")
	cmd.Flags().StringVar(&downloadDir, "download-dir", defaults.DownloadDir, "Default invoice XML download directory")
	cmd.Flags().StringVar(&pdfDir, "pdf-dir", defaults.PDFDir, "Default invoice PDF output directory")
	cmd.Flags().StringVar(&exportDir, "export-dir", defaults.ExportDir, "Default CSV export directory")
	return cmd
}

func challengeCommand() *cobra.Command {
	var outputFile string

	cmd := &cobra.Command{
		Use:   "challenge",
		Short: "Get auth challenge and write unsigned auth request XML",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Challenge(outputFile)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "out", "o", "", "Output path for unsigned XML (default: settings auth_request_file)")
	return cmd
}

func authorizeCommand() *cobra.Command {
	var filePath string
	var verifyCertificateChain bool

	cmd := &cobra.Command{
		Use:   "authorize",
		Short: "Submit signed XAdES XML and save authentication token/reference number",
		RunE: func(cmd *cobra.Command, args []string) error {
			var verify *bool
			if cmd.Flags().Changed("verify-certificate-chain") {
				verify = &verifyCertificateChain
			}
			return commands.Authorize(filePath, verify)
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Signed XML file (default: settings signed_auth_request_file)")
	cmd.Flags().BoolVar(&verifyCertificateChain, "verify-certificate-chain", false, "Verify certificate chain on /auth/xades-signature")
	return cmd
}

func authStatusCommand() *cobra.Command {
	var authenticationToken string
	var referenceNumber string
	var wait bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:     "get-auth-status",
		Aliases: []string{"auth-status"},
		Short:   "Get authentication status using saved or explicit auth operation data",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.GetAuthStatus(authenticationToken, referenceNumber, wait, timeout)
		},
	}

	cmd.Flags().StringVar(&authenticationToken, "authenticationToken", "", "Authentication token")
	cmd.Flags().StringVar(&referenceNumber, "referenceNumber", "", "Reference number")
	cmd.Flags().BoolVar(&wait, "wait", false, "Poll until auth status is no longer pending")
	cmd.Flags().DurationVar(&timeout, "timeout", 60*time.Second, "Polling timeout when --wait is used")
	return cmd
}

func redeemCommand() *cobra.Command {
	var authenticationToken string

	cmd := &cobra.Command{
		Use:   "redeem",
		Short: "Redeem authentication token for access and refresh tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Redeem(authenticationToken)
		},
	}

	cmd.Flags().StringVar(&authenticationToken, "authenticationToken", "", "Authentication token")
	return cmd
}

func refreshCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Refresh access token or re-authenticate with saved KSeF token if needed",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Refresh()
		},
	}
}

func generateTokenCommand() *cobra.Command {
	var description string
	var permissions []string
	var force bool

	cmd := &cobra.Command{
		Use:   "generate-token",
		Short: "Generate a reusable KSeF token",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.GenerateToken(description, permissions, force)
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Token description (default: settings token_description)")
	cmd.Flags().StringArrayVar(&permissions, "permission", nil, "Token permission, repeatable (default: settings token_permissions)")
	cmd.Flags().BoolVar(&force, "force", false, "Generate a new token even if one is already saved")
	return cmd
}

func tokenAuthCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "token-auth",
		Short: "Authenticate with the saved KSeF token and save access/refresh tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.TokenAuth()
		},
	}
}

func listInvoicesCommand() *cobra.Command {
	var month string
	var output string

	cmd := &cobra.Command{
		Use:   "list-invoices",
		Short: "List purchase invoices available for a month",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.ListInvoices(month, output)
		},
	}

	cmd.Flags().StringVarP(&month, "month", "m", "", "Month in YYYY-MM format")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table, json, csv")
	return cmd
}

func listLastMonthCommand() *cobra.Command {
	var output string
	var subject string

	cmd := &cobra.Command{
		Use:   "list-last-month",
		Short: "List last month's purchase and sales invoices",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.ListLastMonth(output, subject)
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table or json")
	cmd.Flags().StringVar(&subject, "subject", "both", "purchase, sales, or both")
	return cmd
}

func downloadCommand() *cobra.Command {
	var month string
	var dir string

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download invoice XMLs for a month",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.Download(month, dir)
		},
	}

	cmd.Flags().StringVarP(&month, "month", "m", "", "Month in YYYY-MM format")
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Output directory (default: settings download_dir)")
	return cmd
}

func downloadPDFsCommand() *cobra.Command {
	var month string
	var dir string
	var subject string
	var force bool
	var keepHTML bool

	cmd := &cobra.Command{
		Use:   "download-pdfs",
		Short: "Render invoice PDF visualizations for a month",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.DownloadPDFs(month, dir, subject, force, keepHTML)
		},
	}

	cmd.Flags().StringVarP(&month, "month", "m", "", "Month in YYYY-MM format")
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Output directory (default: settings pdf_dir)")
	cmd.Flags().StringVar(&subject, "subject", "both", "purchase, sales, or both")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing PDFs")
	cmd.Flags().BoolVar(&keepHTML, "keep-html", false, "Keep the intermediate HTML next to each PDF")
	return cmd
}

func downloadLastMonthPDFsCommand() *cobra.Command {
	var dir string
	var subject string
	var force bool
	var keepHTML bool

	cmd := &cobra.Command{
		Use:   "download-last-month-pdfs",
		Short: "Render last month's invoice PDF visualizations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.DownloadLastMonthPDFs(dir, subject, force, keepHTML)
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Output directory (default: settings pdf_dir)")
	cmd.Flags().StringVar(&subject, "subject", "both", "purchase, sales, or both")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing PDFs")
	cmd.Flags().BoolVar(&keepHTML, "keep-html", false, "Keep the intermediate HTML next to each PDF")
	return cmd
}

func exportCSVCommand() *cobra.Command {
	var month string
	var dir string
	var subject string

	cmd := &cobra.Command{
		Use:   "export-csv",
		Short: "Export invoice metadata to CSV files for a month",
		RunE: func(cmd *cobra.Command, args []string) error {
			return commands.ExportCSV(month, dir, subject)
		},
	}

	cmd.Flags().StringVarP(&month, "month", "m", "", "Month in YYYY-MM format")
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Output directory (default: settings export_dir)")
	cmd.Flags().StringVar(&subject, "subject", "both", "purchase, sales, or both")
	return cmd
}
