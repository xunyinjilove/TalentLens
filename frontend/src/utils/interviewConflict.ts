import type { RecruitmentInterview } from '../composables/useRecruitmentStore'

export type ConflictKind = 'candidate' | 'interviewer' | 'time'

export interface InterviewConflictInput {
  id?: string
  projectCandidateId?: string
  candidateName?: string
  interviewer?: string
  startsAt?: string
  endsAt?: string
  status?: string
}

export interface InterviewConflictItem {
  interview: RecruitmentInterview
  kind: ConflictKind
  labels: string[]
  timeRangeText: string
  summaryText: string
}

function parseTimestamp(value?: string): number {
  if (!value) return Number.NaN
  return Date.parse(value)
}

function pad2(n: number): string {
  return String(n).padStart(2, '0')
}

export function formatConflictTimeRange(startsAt: string, endsAt: string): string {
  const s = new Date(startsAt)
  const e = new Date(endsAt)
  if (Number.isNaN(s.getTime())) return startsAt || ''
  const sDate = `${pad2(s.getMonth() + 1)}-${pad2(s.getDate())}`
  const sTime = `${pad2(s.getHours())}:${pad2(s.getMinutes())}`
  if (Number.isNaN(e.getTime())) return `${sDate} ${sTime}`
  const eDate = `${pad2(e.getMonth() + 1)}-${pad2(e.getDate())}`
  const eTime = `${pad2(e.getHours())}:${pad2(e.getMinutes())}`
  return sDate === eDate ? `${sDate} ${sTime}-${eTime}` : `${sDate} ${sTime} 至 ${eDate} ${eTime}`
}

function normalizeName(value?: string): string {
  return (value || '').trim().toLowerCase()
}

function isPlaceholderInterviewer(value: string): boolean {
  return !value || value === '待指定' || value === '待定'
}

/**
 * 检测给定面试时间段与现有已安排面试列表是否存在时间冲突。
 * 采用区间严格交集判定：startA < endB && startB < endA（首尾相接如 10:00-11:00 与 11:00-12:00 不算冲突）。
 */
export function detectInterviewConflicts(
  current: InterviewConflictInput,
  allInterviews: RecruitmentInterview[]
): InterviewConflictItem[] {
  if (current.status === 'cancelled' || current.status === 'completed') {
    return []
  }
  const startA = parseTimestamp(current.startsAt)
  const endA = parseTimestamp(current.endsAt)
  if (Number.isNaN(startA) || Number.isNaN(endA) || endA <= startA) {
    return []
  }

  const curId = (current.id || '').trim()
  const curCandidateId = (current.projectCandidateId || '').trim()
  const curCandidateName = normalizeName(current.candidateName)
  const curInterviewer = normalizeName(current.interviewer)

  const conflicts: InterviewConflictItem[] = []

  for (const other of allInterviews) {
    if (!other) continue
    if (curId && other.id === curId) continue
    if (other.status === 'cancelled' || other.status === 'completed') continue

    const startB = parseTimestamp(other.startsAt)
    const endB = parseTimestamp(other.endsAt)
    if (Number.isNaN(startB) || Number.isNaN(endB) || endB <= startB) continue

    // 区间重叠核心公式：startA < endB && startB < endA
    if (startA < endB && startB < endA) {
      const otherCandidateId = (other.projectCandidateId || '').trim()
      const otherCandidateName = normalizeName(other.candidateName)
      const otherInterviewer = normalizeName(other.interviewer)

      const sameCandidate =
        (curCandidateId !== '' && curCandidateId === otherCandidateId) ||
        (!curCandidateId && curCandidateName !== '' && curCandidateName === otherCandidateName)

      const sameInterviewer =
        !isPlaceholderInterviewer(curInterviewer) &&
        !isPlaceholderInterviewer(otherInterviewer) &&
        curInterviewer === otherInterviewer

      const labels: string[] = []
      let kind: ConflictKind = 'time'

      if (sameCandidate) {
        labels.push('同一候选人冲突')
        kind = 'candidate'
      }
      if (sameInterviewer) {
        labels.push('同一面试官冲突')
        if (kind !== 'candidate') {
          kind = 'interviewer'
        }
      }
      if (labels.length === 0) {
        labels.push('全局时段重叠')
        kind = 'time'
      }

      const timeRangeText = formatConflictTimeRange(other.startsAt, other.endsAt)
      const interviewerText = (other.interviewer || '').trim() || '待指定'
      const roleText = (other.projectName || '').trim() || '未指定岗位'
      const summaryText = `【${labels.join(' / ')}】${other.candidateName || '候选人'} · ${roleText}（面试官：${interviewerText}，${timeRangeText}）`

      conflicts.push({
        interview: other,
        kind,
        labels,
        timeRangeText,
        summaryText
      })
    }
  }

  // 按严重程度排序：同一候选人冲突 > 同一面试官冲突 > 全局时段重叠，再按开始时间排序
  const weight: Record<ConflictKind, number> = { candidate: 1, interviewer: 2, time: 3 }
  conflicts.sort((a, b) => {
    if (weight[a.kind] !== weight[b.kind]) {
      return weight[a.kind] - weight[b.kind]
    }
    return a.interview.startsAt.localeCompare(b.interview.startsAt)
  })

  return conflicts
}

export function buildConflictConfirmMessage(conflicts: InterviewConflictItem[]): string {
  const lines = conflicts.map((c, idx) => `${idx + 1}. ${c.summaryText}`)
  return [
    `检测到当前时间段与以下 ${conflicts.length} 场已安排面试存在时间冲突：`,
    '',
    ...lines,
    '',
    '注意：时段重叠可能导致候选人分身乏术、面试官撞期或腾讯会议账号并发冲突。是否仍要保存该面试安排？'
  ].join('\n')
}
