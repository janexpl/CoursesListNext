export const MAX_NOTIFICATION_RECIPIENTS = 10

const emailPattern = /^[^\s@,]+@[^\s@,]+$/

export function parseNotificationEmails(value: string): string[] {
  const seen = new Set<string>()

  return value
    .split(',')
    .map(email => email.trim())
    .filter((email) => {
      if (!email) {
        return false
      }

      const key = email.toLocaleLowerCase()
      if (seen.has(key)) {
        return false
      }

      seen.add(key)
      return true
    })
}

export function normalizeNotificationEmails(value: string): string | null {
  const recipients = parseNotificationEmails(value)
  return recipients.length > 0 ? recipients.join(',') : null
}

export function getNotificationEmailsError(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) {
    return ''
  }

  const entries = trimmed.split(',').map(email => email.trim())
  if (entries.some(email => !email || !emailPattern.test(email))) {
    return 'Wpisz poprawne adresy e-mail oddzielone przecinkami.'
  }

  if (parseNotificationEmails(trimmed).length > MAX_NOTIFICATION_RECIPIENTS) {
    return `Możesz podać maksymalnie ${MAX_NOTIFICATION_RECIPIENTS} adresów e-mail.`
  }

  return ''
}
