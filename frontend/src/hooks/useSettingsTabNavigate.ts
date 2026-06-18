import { useNavigate } from 'react-router-dom'
import { settingsTabs } from '@/locales/zh-CN'

/** 项目设置页 Tabs 路由切换 */
export function useSettingsTabNavigate(projectId: string) {
  const navigate = useNavigate()
  return (key: string) => {
    const tab = settingsTabs(projectId).find((t) => t.key === key)
    if (tab) navigate(tab.to)
  }
}
