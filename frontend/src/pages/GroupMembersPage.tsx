import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Alert, AutoComplete, Button, Modal, Select, Spin, Table } from 'antd'
import * as groupsApi from '@/api/groups'
import { memberRoleOptions, roleLabels } from '@/api/projects'
import type { Group, GroupMember } from '@/api/groups'
import type { MemberRole } from '@/api/projects'
import type { User } from '@/api/auth'
import { useGroupPermissions } from '@/hooks/useGroupPermissions'
import { useUserSearch } from '@/hooks/useUserSearch'

export default function GroupMembersPage() {
  const { id: groupId = '' } = useParams()
  const [group, setGroup] = useState<Group | null>(null)
  const [members, setMembers] = useState<GroupMember[]>([])
  const [role, setRole] = useState<MemberRole>('developer')
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [removeOpen, setRemoveOpen] = useState(false)
  const [removeTarget, setRemoveTarget] = useState<GroupMember | null>(null)
  const userSearch = useUserSearch(searchQuery, setSearchQuery)

  const { canManageMembers } = useGroupPermissions(group)

  const roleOptionsWithHints = memberRoleOptions.map((r) => ({
    value: r.value,
    label: `${r.label} — ${r.hint}`,
    title: r.hint,
  }))

  const roleOptions = memberRoleOptions.map((r) => ({
    value: r.value,
    label: r.label,
    title: r.hint,
  }))

  async function load() {
    setLoading(true)
    setError('')
    try {
      const [g, m] = await Promise.all([
        groupsApi.getGroup(groupId),
        groupsApi.listGroupMembers(groupId),
      ])
      setGroup(g)
      setMembers(m.members)
    } catch (e) {
      setError(e instanceof Error ? e.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [groupId])

  async function add() {
    if (!selectedUser) return
    setError('')
    try {
      await groupsApi.addGroupMember(groupId, { user_id: selectedUser.id, role })
      setSelectedUser(null)
      setSearchQuery('')
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : '添加失败')
    }
  }

  async function changeRole(m: GroupMember, newRole: MemberRole) {
    setError('')
    try {
      await groupsApi.updateGroupMember(groupId, m.user_id, newRole)
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : '更新角色失败')
    }
  }

  async function confirmRemove() {
    if (!removeTarget) return
    setError('')
    try {
      await groupsApi.removeGroupMember(groupId, removeTarget.user_id)
      setRemoveOpen(false)
      await load()
    } catch (e) {
      setError(e instanceof Error ? e.message : '移除失败')
    }
  }

  return (
    <div>
      <nav style={{ fontSize: 14, marginBottom: 12, color: 'var(--matrix-text-color-subtle)' }}>
        <Link to="/groups">组</Link>
        <span style={{ margin: '0 6px' }}>/</span>
        <span>{group?.name || '…'}</span>
        <span style={{ margin: '0 6px' }}>/</span>
        <span>成员</span>
      </nav>
      <h2>{group?.name || '组成员'}</h2>
      {error && <Alert type="error" message={error} style={{ marginBottom: 16 }} />}
      {canManageMembers && (
        <div className="panel stack" style={{ maxWidth: 480, marginBottom: 20 }}>
          <h3>添加成员</h3>
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
              已选择：<strong>{selectedUser.name || selectedUser.username}</strong>
            </p>
          )}
          <Select value={role} onChange={setRole} style={{ minWidth: 140 }} options={roleOptionsWithHints} />
          <Button type="primary" disabled={!selectedUser} onClick={add}>添加成员</Button>
        </div>
      )}
      {!canManageMembers && !loading && (
        <Alert type="info" message="您仅有查看权限。如需管理成员，请联系组 Maintainer 或 Owner。" style={{ marginBottom: 16 }} />
      )}
      {loading ? (
        <p className="muted">加载中…</p>
      ) : (
        <Table dataSource={members} rowKey="user_id" pagination={false}>
          <Table.Column title="账号" render={(_, row: GroupMember) => (
            <>
              <div>{row.name || row.username}</div>
              <div className="muted">@{row.username}</div>
            </>
          )} />
          <Table.Column title="邮箱" dataIndex="email" />
          <Table.Column title="角色" render={(_, row: GroupMember) => (
            canManageMembers ? (
              <Select
                value={row.role}
                onChange={(r) => changeRole(row, r)}
                style={{ minWidth: 140 }}
                options={roleOptions}
              />
            ) : roleLabels[row.role]
          )} />
          <Table.Column title="加入时间" render={(_, row: GroupMember) => (
            new Date(row.created_at).toLocaleDateString()
          )} />
          {canManageMembers && (
            <Table.Column title="操作" render={(_, row: GroupMember) => (
              <Button type="link" danger onClick={() => { setRemoveTarget(row); setRemoveOpen(true) }}>移除</Button>
            )} />
          )}
        </Table>
      )}
      <Modal open={removeOpen} title="移除成员" onCancel={() => setRemoveOpen(false)} onOk={confirmRemove} okText="移除" okButtonProps={{ danger: true }}>
        确定将 <strong>{removeTarget?.name || removeTarget?.username}</strong> 从组中移除？
      </Modal>
    </div>
  )
}
