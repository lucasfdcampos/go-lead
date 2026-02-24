package cnpj

import (
"context"
"fmt"
"strings"
"time"

"github.com/chromedp/chromedp"
)

type ChromeDPSearcher struct {
Headless bool
}

func NewChromeDPSearcher(headless bool) *ChromeDPSearcher {
return &ChromeDPSearcher{Headless: headless}
}

func (c *ChromeDPSearcher) Name() string {
return "Web Scraping (ChromeDP + Google)"
}

func (c *ChromeDPSearcher) Search(ctx context.Context, query string) (*CNPJ, error) {
opts := []chromedp.ExecAllocatorOption{
chromedp.NoFirstRun,
chromedp.NoDefaultBrowserCheck,
chromedp.DisableGPU,
chromedp.NoSandbox,
}

if c.Headless {
opts = append(opts, chromedp.Headless)
}

allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
defer cancel()

ctx, cancel = chromedp.NewContext(allocCtx)
defer cancel()

ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
defer cancel()

searchURL := fmt.Sprintf("https://www.google.com/search?q=%s", strings.ReplaceAll(query, " ", "+"))

var htmlContent string

err := chromedp.Run(ctx,
chromedp.Navigate(searchURL),
chromedp.Sleep(2*time.Second),
chromedp.OuterHTML("body", &htmlContent),
)

if err != nil {
return nil, fmt.Errorf("erro ao fazer scraping: %w", err)
}

cnpjs := ExtractAllCNPJs(htmlContent)
if len(cnpjs) > 0 {
return cnpjs[0], nil
}

return nil, fmt.Errorf("CNPJ não encontrado no scraping")
}

type GoogleScrapingSearcher = ChromeDPSearcher

func NewGoogleScrapingSearcher(headless bool) *GoogleScrapingSearcher {
return NewChromeDPSearcher(headless)
}
