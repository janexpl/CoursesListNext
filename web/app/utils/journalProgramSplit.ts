type ProgramEntry = { TheoryTime?: string, PracticeTime?: string }
type Session = { id: number, sortOrder: number, hours: number }

export function formatProgramHours(value: string | number | undefined) {
  const parsed = Number.parseFloat(`${value ?? ''}`.trim().replace(',', '.'))

  if (!Number.isFinite(parsed)) {
    return '0'
  }

  return String(Math.round(parsed * 100) / 100)
}

// Godziny teorii i praktyki dla każdej sesji wydruku. Temat podzielony na kilka dni ma kilka
// sesji z tym samym sortOrder; każda dostaje swoją część godzin, najpierw teorię, potem
// praktykę - inaczej wydruk liczyłby temat podwójnie. Ta sama reguła co w pdf.go po stronie API.
export function journalProgramSplitBySession(sessions: Session[], entries: ProgramEntry[]) {
  const map: Record<number, { theory: string, practice: string }> = {}
  const partsBySortOrder = new Map<number, number>()
  const theoryLeft = new Map<number, number>()

  for (const session of sessions) {
    partsBySortOrder.set(session.sortOrder, (partsBySortOrder.get(session.sortOrder) ?? 0) + 1)
  }

  for (const session of sessions) {
    const entry = entries[session.sortOrder - 1]

    if (!entry) {
      map[session.id] = { theory: formatProgramHours(session.hours), practice: '0' }
      continue
    }

    if (partsBySortOrder.get(session.sortOrder) === 1) {
      map[session.id] = {
        theory: formatProgramHours(entry.TheoryTime),
        practice: formatProgramHours(entry.PracticeTime)
      }
      continue
    }

    const left = theoryLeft.get(session.sortOrder)
      ?? Math.max(Number.parseFloat(`${entry.TheoryTime ?? ''}`.replace(',', '.')) || 0, 0)
    const hours = Number(session.hours)
    const theory = Math.min(left, hours)
    theoryLeft.set(session.sortOrder, left - theory)
    map[session.id] = {
      theory: formatProgramHours(theory),
      practice: formatProgramHours(hours - theory)
    }
  }

  return map
}
