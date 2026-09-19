// Podglądy szablonu w edytorach kursu pokazują sam szablon, bez danych konkretnego
// zaświadczenia - nie ma tam więc kodu weryfikacyjnego ani gotowego obrazka QR.
// Znacznik {{ kod_qr }} renderujemy jako szary kwadrat tej samej wielkości co
// prawdziwy kod (24 mm), żeby autor szablonu widział, ile miejsca zajmie na wydruku.

export const templatePreviewQrCss = `
      .qr-preview {
        width: 24mm;
        height: 24mm;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        border: 1px dashed #94a3b8;
        border-radius: 2px;
        background: #f1f5f9;
        color: #64748b;
        font-family: ui-sans-serif, system-ui, sans-serif;
        font-size: 9px;
        letter-spacing: 0.12em;
        text-align: center;
        line-height: 1.2;
      }`

const qrPlaceholderPattern = /{{\s*kod_qr\s*}}/g

const qrPreviewBox = '<span class="qr-preview">KOD QR</span>'

// renderTemplatePreviewQr podmienia znacznik kodu QR na zastępczy kwadrat.
// Pozostałe znaczniki zostają nietknięte - w podglądzie szablonu widać je tak,
// jak zostały wpisane.
export function renderTemplatePreviewQr(html: string) {
  return html.replace(qrPlaceholderPattern, qrPreviewBox)
}
