package render

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	ksef "github.com/oskar1233/ksef/internal"
)

type PDFRenderOptions struct {
	HTMLPath string
}

type PDFGeneratorFunc func(ctx context.Context, htmlPath string, outputPath string) error

var pdfGenerator PDFGeneratorFunc = generatePDFWithChromedp

func SetPDFGeneratorForTesting(generator PDFGeneratorFunc) func() {
	previous := pdfGenerator
	pdfGenerator = generator
	return func() {
		pdfGenerator = previous
	}
}

func RenderInvoicePDF(invoice ksef.InvoiceMetadata, xmlContent []byte, subjectLabel string, outputPath string) error {
	return RenderInvoicePDFWithOptions(invoice, xmlContent, subjectLabel, outputPath, PDFRenderOptions{})
}

func RenderInvoicePDFWithOptions(invoice ksef.InvoiceMetadata, xmlContent []byte, subjectLabel string, outputPath string, options PDFRenderOptions) error {
	htmlContent, err := RenderInvoiceHTML(invoice, xmlContent, subjectLabel)
	if err != nil {
		return err
	}

	htmlPath, cleanup, err := resolveHTMLPath(outputPath, options.HTMLPath)
	if err != nil {
		return err
	}
	defer cleanup()

	if err := os.MkdirAll(filepath.Dir(htmlPath), 0o755); err != nil {
		return fmt.Errorf("create html directory: %w", err)
	}
	if err := os.WriteFile(htmlPath, htmlContent, 0o644); err != nil {
		return fmt.Errorf("write html %s: %w", htmlPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create pdf directory: %w", err)
	}
	if err := pdfGenerator(context.Background(), htmlPath, outputPath); err != nil {
		return fmt.Errorf("render pdf %s: %w", outputPath, err)
	}
	return nil
}

func resolveHTMLPath(outputPath string, explicitHTMLPath string) (string, func(), error) {
	if explicitHTMLPath != "" {
		return explicitHTMLPath, func() {}, nil
	}

	tempDir, err := os.MkdirTemp("", "ksef-render-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp render directory: %w", err)
	}

	baseName := filepath.Base(outputPath)
	htmlPath := filepath.Join(tempDir, baseName[:len(baseName)-len(filepath.Ext(baseName))]+".html")
	return htmlPath, func() {
		_ = os.RemoveAll(tempDir)
	}, nil
}

func generatePDFWithChromedp(ctx context.Context, htmlPath string, outputPath string) error {
	allocatorOptions := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)
	if browserPath := findChromeBinary(); browserPath != "" {
		allocatorOptions = append(allocatorOptions, chromedp.ExecPath(browserPath))
	}

	allocCtx, cancelAllocator := chromedp.NewExecAllocator(ctx, allocatorOptions...)
	defer cancelAllocator()

	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()

	var pdfData []byte
	if err := chromedp.Run(taskCtx,
		chromedp.Navigate(fileURL(htmlPath)),
		chromedp.WaitReady("body", chromedp.ByQuery),
		emulation.SetEmulatedMedia().WithMedia("print"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfData, _, err = page.PrintToPDF().
				WithPrintBackground(true).
				WithPreferCSSPageSize(true).
				WithMarginTop(0).
				WithMarginBottom(0).
				WithMarginLeft(0).
				WithMarginRight(0).
				Do(ctx)
			return err
		}),
	); err != nil {
		return fmt.Errorf("chromedp print: %w. set KSEF_CHROME_PATH if Chrome is installed in a non-standard location", err)
	}

	if err := os.WriteFile(outputPath, pdfData, 0o644); err != nil {
		return fmt.Errorf("write pdf file: %w", err)
	}
	return nil
}

func findChromeBinary() string {
	for _, envName := range []string{"KSEF_CHROME_PATH", "CHROME_PATH"} {
		if value := os.Getenv(envName); value != "" {
			return value
		}
	}

	for _, candidate := range []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"chrome",
	} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}

	for _, candidate := range []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

func fileURL(path string) string {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		absolutePath = path
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(absolutePath)}).String()
}
