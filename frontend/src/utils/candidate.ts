/**
 * 提取候选人的真实姓氏（过滤【前程无忧】、【BOSS直聘】、文件名后缀、称谓等）
 */
export function getCandidateSurname(candidateOrName: any): string {
  if (!candidateOrName) return '人'
  const rawName = typeof candidateOrName === 'string'
    ? candidateOrName
    : (candidateOrName.name || candidateOrName.fileName || candidateOrName.candidateName || '')

  if (!rawName) return '人'

  // 1. 过滤开头的平台标签方括号或圆括号，如【前程无忧】、【BOSS直聘】、[智联]、(猎聘)
  let cleaned = rawName.replace(/^\s*[【\[(（].*?[】\])）]\s*/g, '').trim()

  // 2. 去除常见文件后缀
  cleaned = cleaned.replace(/\.(txt|pdf|docx?|html?|png|jpg)$/i, '').trim()

  // 3. 去除下划线、空格、短横线后面的内容（如 "夏女士_临床项目经理" -> "夏女士"）
  const namePart = cleaned.split(/[_–—\s\-]+/)[0].trim() || cleaned

  // 4. 处理称谓后缀（如 "许先生" -> "许"，"夏女士" -> "夏"，"王老师" -> "王"，"李博士" -> "李"）
  const honorificMatch = namePart.match(/^(.+?)(?:先生|女士|小姐|老师|博士|教授|专家|同学)$/)
  if (honorificMatch && honorificMatch[1]) {
    const prefix = honorificMatch[1]
    return prefix.length > 2 ? prefix.slice(0, 2) : prefix.slice(0, 1)
  }

  // 5. 常见复姓（诸葛、欧阳、司马、上官、皇甫、令狐、慕容等）
  const compoundSurnames = ['诸葛', '欧阳', '司马', '上官', '皇甫', '令狐', '慕容', '西门', '公孙', '独孤', '宇文']
  for (const cs of compoundSurnames) {
    if (namePart.startsWith(cs)) {
      return cs
    }
  }

  // 6. 如果是英文名（如 "David" -> "D"），转大写首字母
  const englishMatch = namePart.match(/^[A-Za-z]/)
  if (englishMatch) {
    return englishMatch[0].toUpperCase()
  }

  // 7. 纯中文单姓直接取第一个汉字（过滤非法标点）
  const firstChar = namePart.charAt(0)
  return /^[\u4e00-\u9fa5]$/.test(firstChar) ? firstChar : (firstChar || '人')
}
