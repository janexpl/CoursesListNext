export type CourseCertificateTranslationProgramEntry = {
  Subject?: string
  TheoryTime?: string
  PracticeTime?: string
}

export type CourseCertificateTranslationProgramRow = {
  id: number
  subject: string
  theoryTime: string
  practiceTime: string
}

export type CourseCertificateTranslationPayload = {
  languageCode: string
  courseName: string
  courseProgram: string
  certFrontPage: string
}

export type CourseCertificateTranslationForm = {
  id: number
  languageCode: string
  courseName: string
  certFrontPage: string
  programRows: CourseCertificateTranslationProgramRow[]
  hasInvalidStoredProgram: boolean
}

export const supportedCourseCertificateTranslationLanguages = [
  { code: 'en', label: 'angielski' },
  { code: 'de', label: 'niemiecki' },
  { code: 'uk', label: 'ukraiński' },
  { code: 'cs', label: 'czeski' },
  { code: 'sk', label: 'słowacki' },
  { code: 'lt', label: 'litewski' }
] as const

const supportedCourseCertificateTranslationLanguageCodes: ReadonlySet<string> = new Set(
  supportedCourseCertificateTranslationLanguages.map(language => language.code)
)

export function normalizeCourseCertificateTranslationLanguageCode(value: string) {
  return value.trim().toLowerCase()
}

export function isSupportedCourseCertificateTranslationLanguageCode(value: string) {
  return supportedCourseCertificateTranslationLanguageCodes.has(
    normalizeCourseCertificateTranslationLanguageCode(value)
  )
}

export function normalizeCourseCertificateTranslationHours(value: string) {
  return value.trim().replace(',', '.')
}

export function isCourseCertificateTranslationHoursValueValid(value: string) {
  return /^\d+(\.\d+)?$/.test(value)
}

export function buildCourseCertificateTranslationProgramEntries(
  rows: CourseCertificateTranslationProgramRow[]
) {
  return rows
    .map((row) => {
      return {
        Subject: row.subject.trim(),
        TheoryTime: normalizeCourseCertificateTranslationHours(row.theoryTime) || '0',
        PracticeTime: normalizeCourseCertificateTranslationHours(row.practiceTime) || '0'
      }
    })
    .filter(row => row.Subject || row.TheoryTime !== '0' || row.PracticeTime !== '0')
}

export function hasInvalidCourseCertificateTranslationProgram(
  rows: CourseCertificateTranslationProgramRow[]
) {
  return buildCourseCertificateTranslationProgramEntries(rows).some((row) => {
    return !row.Subject || !isCourseCertificateTranslationHoursValueValid(row.TheoryTime ?? '0') || !isCourseCertificateTranslationHoursValueValid(row.PracticeTime ?? '0')
  })
}

export function serializeCourseCertificateTranslationProgramRows(
  rows: CourseCertificateTranslationProgramRow[]
) {
  return JSON.stringify(buildCourseCertificateTranslationProgramEntries(rows))
}

export function parseCourseCertificateTranslationProgramRows(
  value: string,
  createRow: (values?: Partial<Omit<CourseCertificateTranslationProgramRow, 'id'>>) => CourseCertificateTranslationProgramRow
) {
  if (!value.trim()) {
    return {
      rows: [] as CourseCertificateTranslationProgramRow[],
      invalid: false
    }
  }

  try {
    const parsed = JSON.parse(value)
    if (!Array.isArray(parsed)) {
      return {
        rows: [] as CourseCertificateTranslationProgramRow[],
        invalid: true
      }
    }

    return {
      rows: parsed.map((entry: CourseCertificateTranslationProgramEntry) => createRow({
        subject: entry.Subject ?? '',
        theoryTime: entry.TheoryTime ?? '',
        practiceTime: entry.PracticeTime ?? ''
      })),
      invalid: false
    }
  } catch {
    return {
      rows: [] as CourseCertificateTranslationProgramRow[],
      invalid: true
    }
  }
}

export function buildCourseCertificateTranslationPayloads(
  translations: CourseCertificateTranslationForm[]
): CourseCertificateTranslationPayload[] {
  return translations.map((translation) => {
    return {
      languageCode: normalizeCourseCertificateTranslationLanguageCode(translation.languageCode),
      courseName: translation.courseName.trim(),
      courseProgram: serializeCourseCertificateTranslationProgramRows(translation.programRows),
      certFrontPage: translation.certFrontPage.trim()
    }
  })
}

export function getCourseCertificateTranslationsValidationError(
  translations: CourseCertificateTranslationForm[]
) {
  const seenLanguages = new Set<string>()

  for (const translation of translations) {
    const languageCode = normalizeCourseCertificateTranslationLanguageCode(translation.languageCode)

    if (!languageCode) {
      return 'Uzupełnij kod języka dla każdej wersji obcojęzycznej.'
    }

    if (languageCode === 'pl') {
      return 'Kod języka pl jest zarezerwowany dla podstawowego szablonu kursu.'
    }

    if (!isSupportedCourseCertificateTranslationLanguageCode(languageCode)) {
      return 'Obsługiwane kody języków to: en, de, uk, cs, sk, lt.'
    }

    if (seenLanguages.has(languageCode)) {
      return 'Każda wersja obcojęzyczna musi mieć unikalny kod języka.'
    }

    seenLanguages.add(languageCode)

    if (!translation.courseName.trim()) {
      return `Uzupełnij nazwę kursu dla języka ${languageCode}.`
    }

    if (translation.hasInvalidStoredProgram) {
      return `Program kursu dla języka ${languageCode} wymaga poprawy przed zapisem.`
    }

    if (!buildCourseCertificateTranslationProgramEntries(translation.programRows).length) {
      return `Dodaj przynajmniej jeden temat programu dla języka ${languageCode}.`
    }

    if (hasInvalidCourseCertificateTranslationProgram(translation.programRows)) {
      return `Popraw temat albo godziny w programie dla języka ${languageCode}.`
    }

    if (!translation.certFrontPage.trim()) {
      return `Uzupełnij szablon zaświadczenia dla języka ${languageCode}.`
    }
  }

  return ''
}

export function isCourseCertificateTranslationReady(
  translation: CourseCertificateTranslationForm,
  translations: CourseCertificateTranslationForm[]
) {
  const languageCode = normalizeCourseCertificateTranslationLanguageCode(translation.languageCode)

  if (!languageCode || languageCode === 'pl') {
    return false
  }

  if (!isSupportedCourseCertificateTranslationLanguageCode(languageCode)) {
    return false
  }

  if (translations.filter(item => normalizeCourseCertificateTranslationLanguageCode(item.languageCode) === languageCode).length > 1) {
    return false
  }

  return !!(
    translation.courseName.trim()
    && !translation.hasInvalidStoredProgram
    && buildCourseCertificateTranslationProgramEntries(translation.programRows).length
    && !hasInvalidCourseCertificateTranslationProgram(translation.programRows)
    && translation.certFrontPage.trim()
  )
}

export function countReadyCourseCertificateTranslations(
  translations: CourseCertificateTranslationForm[]
) {
  return translations.filter(translation => isCourseCertificateTranslationReady(translation, translations)).length
}
