// Podglądy szablonu w edytorach kursu pokazują sam szablon, bez danych konkretnego
// zaświadczenia - nie ma tam ani kodu weryfikacyjnego, ani wgranych pieczątek.
// Znaczniki, które na wydruku stają się obrazkami, renderujemy jako szare prostokąty
// tej samej wielkości co prawdziwe nadruki, żeby autor szablonu widział, ile miejsca zajmą.

export const templatePreviewQrCss = `
      .qr-preview,
      .decor-preview {
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
      }

      /* Wyższy niż sam kod: pod kwadratem drukuje się jeszcze podpis "Sprawdź ważność". */
      .qr-preview {
        width: 24mm;
        height: 28mm;
      }

      .stamp-preview {
        width: 40mm;
        height: 22mm;
      }

      /* Pieczęć okrągła jest kwadratowa i okrągła - reszta prostokątna. */
      .stamp-preview--round {
        width: 30mm;
        height: 30mm;
        border-radius: 50%;
      }

      .signature-preview {
        width: 45mm;
        height: 18mm;
      }

      /* Podstawa prawna to tekst, a nie obrazek: w podglądzie widać jej prawdziwą treść,
         lekko podkreśloną, żeby autor szablonu wiedział, skąd się wzięła. */
      .legal-basis-preview {
        text-decoration: underline dotted #0284c7;
        text-underline-offset: 3px;
      }

      .legal-basis-preview--missing {
        padding: 0 4px;
        border: 1px dashed #f59e0b;
        background: #fffbeb;
        color: #b45309;
        font-family: ui-sans-serif, system-ui, sans-serif;
        font-size: 9px;
        letter-spacing: 0.08em;
        text-decoration: none;
      }`

// Rozmiary odpowiadają domyślnym szerokościom nadruków z API (certassets.DefaultWidthMM).
const previewBoxes: Array<{ token: RegExp, html: string }> = [
  { token: /{{\s*kod_qr\s*}}/g, html: '<span class="qr-preview">KOD QR</span>' },
  { token: /{{\s*pieczatka_okragla\s*}}/g, html: '<span class="decor-preview stamp-preview stamp-preview--round">PIECZĄTKA OKRĄGŁA</span>' },
  { token: /{{\s*pieczatka_firmowa\s*}}/g, html: '<span class="decor-preview stamp-preview">PIECZĄTKA FIRMOWA</span>' },
  { token: /{{\s*pieczatka_imienna\s*}}/g, html: '<span class="decor-preview stamp-preview">PIECZĄTKA IMIENNA</span>' },
  { token: /{{\s*podpis\s*}}/g, html: '<span class="decor-preview signature-preview">PODPIS</span>' }
]

function escapePreviewText(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
}

const legalBasisToken = /{{\s*podstawa_prawna\s*}}/gi

// renderTemplatePreviewQr podmienia znaczniki obrazkowe na zastępcze prostokąty,
// a {{ podstawa_prawna }} - na treść podstawy wybranej w ustawieniach kursu (albo na
// ostrzeżenie, gdy kurs żadnej nie wskazuje: zaświadczenie miałoby w tym miejscu pustkę).
// Pozostałe znaczniki zostają nietknięte - w podglądzie szablonu widać je tak,
// jak zostały wpisane.
export function renderTemplatePreviewQr(html: string, legalBasis?: string | null) {
  const withBoxes = previewBoxes.reduce((result, box) => result.replace(box.token, box.html), html)
  const basis = legalBasis?.trim()
  return withBoxes.replace(legalBasisToken, basis
    ? `<span class="legal-basis-preview">${escapePreviewText(basis)}</span>`
    : '<span class="legal-basis-preview legal-basis-preview--missing">PODSTAWA PRAWNA — WYBIERZ W USTAWIENIACH KURSU</span>')
}

// Czy szablon używa znacznika podstawy prawnej - do ostrzeżeń w formularzu kursu.
export function templateUsesLegalBasis(html: string) {
  return /{{\s*podstawa_prawna\s*}}/i.test(html)
}
