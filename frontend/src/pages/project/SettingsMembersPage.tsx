import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Alert, AutoComplete, Avatar, Button, Form, Select, Spin, Table, Tabs } from 'antd'
import { useProjectStore } from '@/stores/project'
import { useProjectPermissions } from '@/hooks/useProjectPermissions'
import { useSettingsTabNavigate } from '@/hooks/useSettingsTabNavigate'
import { useUserSearch } from '@/hooks/useUserSearch'
import * as projectsApi from '@/api/projects'
import type { ProjectMember, MemberRole } from '@/api/projects'
import type { User } from '@/api/auth'
import { settingsTabs } from '@/locales/zh-CN'
import { avatarInitials } from '@/utils/avatar'

export default function SettingsMembersPage() {
  const { id = '' } = useParams()
  const projectStore = useProjectStore()
  const perms = useProjectPermissions(projectStore.current)
  const onSettingsTabChange = useSettingsTabNavigate(id)
  const [members, setMembers] = useState<ProjectMember[]>([])
  const [username, setUsername] = useState('')
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [role, setRole] = useState<MemberRole>('developer')
  const [error, setError] = useState('')
  const userSearch = useUserSearch(username, setUsername)

  async function load() {
    const res = await projectsApi.listMembers(id)
    setMembers(res.members)
  }

  useEffect(() => { load() }, [id])

  async function invite() {
    setError('')
    const name = username.trim()
    if (!name) return
    try {
      await projectsApi.addMember(id, { username: name, role })
      setUsername('')
      setSelectedUser(null)
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : '添加失败')
    }
  }

  async function changeRole(m: ProjectMember, newRole: MemberRole) {
    await projectsApi.updateMember(id, m.user_id, newRole)
    await load()
  }

  async function remove(m: ProjectMember) {
    await projectsApi.removeMember(id, m.user_id)
    await load()
  }

  const roleOptions = projectsApi.memberRoleOptions.map((r) => ({
    value: r.value,
    label: `${r.label} — ${r.hint}`,
    title: r.hint,
  }))

  return (
    <div>
      <Tabs
        activeKey="members"
        onChange={onSettingsTabChange}
        items={settingsTabs(id).map((tab) => ({ key: tab.key, label: tab.label }))}
        style={{ marginBottom: 16 }}
      />
      <h2>项目成员</h2>
      {error && <Alert type="error" message={error} style={{ marginBottom: 16 }} />}
      {!perms.canManageMembers && !error && (
        <Alert type="info" message="您仅有查看权限。如需邀请或管理成员，请联系项目 Maintainer 或 Owner。" style={{ marginBottom: 16 }} />
      )}
      {perms.canManageMembers && (
        <div className="panel stack" style={{ maxWidth: 560, marginBottom: 16 }}>
          <h3>邀请成员</h3>
          <p className="muted">按登录名、显示名称或邮箱搜索。</p>
          <AutoComplete
            value={userSearch.query}
            options={userSearch.options}
            onSelect={(val) => userSearch.pick(val, setSelectedUser)}
            onSearch={userSearch.onSearchInput}
            placeholder="按登录名、显示名称或邮箱搜索"
            notFoundContent={userSearch.loading ? <Spin size="small" /> : null}
            style={{ width: '100%' }}
          />
          {selectedUser && (
            <p style={{ fontSize: 13, margin: 0 }}>
              已选择：<strong>{selectedUser.name || selectedUser.username}</strong> (@{selectedUser.username})
            </p>
          )}
          <Form.Item label="角色">
            <Select value={role} onChange={setRole} style={{ minWidth: 140 }} options={roleOptions} />
          </Form.Item>
          <Button type="primary" disabled={!username.trim()} onClick={invite}>邀请</Button>
        </div>
      )}
      <Table dataSource={members} rowKey="user_id" pagination={false}>
        <Table.Column title="" width={48} render={(_, row: ProjectMember) => (
          <Avatar style={{ backgroundColor: '#fc6d26' }}>{avatarInitials(row.name || row.username)}</Avatar>
        )} />
        <Table.Column title="账号" render={(_, row: ProjectMember) => (
          <>
            <div>{row.name || row.username}</div>
            <div className="muted">@{row.username}</div>
          </>
        )} />
        <Table.Column title="角色" render={(_, row: ProjectMember) => (
          perms.canManageMembers ? (
            <Select
              value={row.role}
              onChange={(r) => changeRole(row, r)}
              style={{ minWidth: 140 }}
              options={projectsApi.memberRoleOptions.map((r) => ({ value: r.value, label: r.label, title: r.hint }))}
            />
          ) : projectsApi.roleLabels[row.role]
        )} />
        <Table.Column title="加入时间" render={(_, row: ProjectMember) => (
          new Date(row.created_at).toLocaleDateString()
        )} />
        {perms.canManageMembers && (
          <Table.Column title="操作" render={(_, row: ProjectMember) => (
            <Button type="link" danger onClick={() => remove(row)}>移除</Button>
          )} />
        )}
      </Table>
    </div>
  )
}
