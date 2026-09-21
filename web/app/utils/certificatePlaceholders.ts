// Znaczniki szablonu zaświadczenia - jedna lista dla wszystkich trzech edytorów:
// tworzenia kursu, edycji kursu i tłumaczeń. Wcześniej każdy miał własną kopię, więc
// dodanie znacznika wymagało pamiętania o trzech plikach.
//
// Lista musi zgadzać się z podmianą po stronie API
// (substituteCertificateTemplate w api/internal/certificates/pdf.go).

export type TemplatePlaceholder = {
  label: string
  value: string
}

export const certificateTemplatePlaceholders: TemplatePlaceholder[] = [
  { label: 'Imię', value: '{{ imie }}' },
  { label: 'Drugie imię', value: '{{ drugie_imie }}' },
  { label: 'Nazwisko', value: '{{ nazwisko }}' },
  { label: 'Data urodzenia', value: '{{ data_urodzenia }}' },
  { label: 'Miejsce urodzenia', value: '{{ miejsce_urodzenia }}' },
  { label: 'Nazwa kursu', value: '{{ nazwa_kursu }}' },
  { label: 'Data rozpoczęcia', value: '{{ data_rozpoczecia }}' },
  { label: 'Data zakończenia', value: '{{ data_zakonczenia }}' },
  { label: 'Data wystawienia', value: '{{ data_wystawienia }}' },
  { label: 'Numer zaświadczenia', value: '{{ numer_zaswiadczenia }}' },
  { label: 'Kod QR', value: '{{ kod_qr }}' },
  // Nadruki pojawiają się wyłącznie na zaświadczeniach wystawianych przez platformę;
  // na pozostałych wydrukach znacznik po prostu znika.
  { label: 'Pieczątka okrągła', value: '{{ pieczatka_okragla }}' },
  { label: 'Pieczątka firmowa', value: '{{ pieczatka_firmowa }}' },
  { label: 'Pieczątka imienna', value: '{{ pieczatka_imienna }}' },
  { label: 'Podpis', value: '{{ podpis }}' }
]
