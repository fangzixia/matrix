import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  Alert,
  AutoComplete,
  Breadcrumb,
  Button,
  Card,
  Modal,
  Select,
  Space,
  Spin,
  Table,
  Typography,
} from "antd";
import * as groupsApi from "@/api/groups";
import { memberRoleOptions, roleLabels } from "@/api/projects";
import type { Group, GroupMember } from "@/api/groups";
import type { MemberRole } from "@/api/projects";
import type { User } from "@/api/auth";
import { useGroupPermissions } from "@/hooks/useGroupPermissions";
import { useUserSearch } from "@/hooks/useUserSearch";

export default function GroupMembersPage() {
  const { id: groupId = "" } = useParams();
  const [group, setGroup] = useState<Group | null>(null);
  const [members, setMembers] = useState<GroupMember[]>([]);
  const [role, setRole] = useState<MemberRole>("developer");
  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [removeOpen, setRemoveOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<GroupMember | null>(null);
  const userSearch = useUserSearch(searchQuery, setSearchQuery);
  const { canManageMembers } = useGroupPermissions(group);
  const roleOptionsWithHints = memberRoleOptions.map((r) => ({
    value: r.value,
    label: `${r.label} — ${r.hint}`,
    title: r.hint,
  }));
  const roleOptions = memberRoleOptions.map((r) => ({
    value: r.value,
    label: r.label,
    title: r.hint,
  }));
  async function load() {
    setLoading(true);
    setError("");
    try {
      const [g, m] = await Promise.all([
        groupsApi.getGroup(groupId),
        groupsApi.listGroupMembers(groupId),
      ]);
      setGroup(g);
      setMembers(m.members);
    } catch (e) {
      setError(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    load();
  }, [groupId]);
  async function add() {
    if (!selectedUser) return;
    setError("");
    try {
      await groupsApi.addGroupMember(groupId, {
        user_id: selectedUser.id,
        role,
      });
      setSelectedUser(null);
      setSearchQuery("");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "添加失败");
    }
  }
  async function changeRole(m: GroupMember, newRole: MemberRole) {
    setError("");
    try {
      await groupsApi.updateGroupMember(groupId, m.user_id, newRole);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "更新角色失败");
    }
  }
  async function confirmRemove() {
    if (!removeTarget) return;
    setError("");
    try {
      await groupsApi.removeGroupMember(groupId, removeTarget.user_id);
      setRemoveOpen(false);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "移除失败");
    }
  }
  return (
    <>
      <Breadcrumb
        style={{ marginBottom: 12 }}
        items={[
          { title: <Link to="/groups">组</Link> },
          { title: group?.name || "…" },
          { title: "成员" },
        ]}
      />
      <Typography.Title level={2}>{group?.name || "组成员"}</Typography.Title>
      {error && (
        <Alert type="error" message={error} style={{ marginBottom: 16 }} />
      )}
      {canManageMembers && (
        <Card title="添加成员" style={{ maxWidth: 480, marginBottom: 20 }}>
          <Space direction="vertical" style={{ width: "100%" }} size="middle">
            <AutoComplete
              value={userSearch.query}
              options={userSearch.options}
              onSelect={(val) => userSearch.pick(val, setSelectedUser)}
              onSearch={userSearch.onSearchInput}
              placeholder="按登录名、显示名称或邮箱搜索"
              notFoundContent={
                userSearch.loading ? <Spin size="small" /> : null
              }
              style={{ width: "100%" }}
            />
            {selectedUser && (
              <Typography.Text style={{ fontSize: 13 }}>
                已选择：
                <strong>{selectedUser.name || selectedUser.username}</strong>
              </Typography.Text>
            )}
            <Select
              value={role}
              onChange={setRole}
              style={{ minWidth: 140 }}
              options={roleOptionsWithHints}
            />
            <Button type="primary" disabled={!selectedUser} onClick={add}>
              添加成员
            </Button>
          </Space>
        </Card>
      )}
      {!canManageMembers && !loading && (
        <Alert
          type="info"
          message="您仅有查看权限。如需管理成员，请联系组 Maintainer 或 Owner。"
          style={{ marginBottom: 16 }}
        />
      )}
      {loading ? (
        <Typography.Text type="secondary">加载中…</Typography.Text>
      ) : (
        <Table dataSource={members} rowKey="user_id" pagination={false}>
          <Table.Column
            title="账号"
            render={(_, row: GroupMember) => (
              <>
                <div>{row.name || row.username}</div>
                <Typography.Text type="secondary">
                  @{row.username}
                </Typography.Text>
              </>
            )}
          />
          <Table.Column title="邮箱" dataIndex="email" />
          <Table.Column
            title="角色"
            render={(_, row: GroupMember) =>
              canManageMembers ? (
                <Select
                  value={row.role}
                  onChange={(r) => changeRole(row, r)}
                  style={{ minWidth: 140 }}
                  options={roleOptions}
                />
              ) : (
                roleLabels[row.role]
              )
            }
          />
          <Table.Column
            title="加入时间"
            render={(_, row: GroupMember) =>
              new Date(row.created_at).toLocaleDateString()
            }
          />
          {canManageMembers && (
            <Table.Column
              title="操作"
              render={(_, row: GroupMember) => (
                <Button
                  type="link"
                  danger
                  onClick={() => {
                    setRemoveTarget(row);
                    setRemoveOpen(true);
                  }}
                >
                  移除
                </Button>
              )}
            />
          )}
        </Table>
      )}
      <Modal
        open={removeOpen}
        title="移除成员"
        onCancel={() => setRemoveOpen(false)}
        onOk={confirmRemove}
        okText="移除"
        okButtonProps={{ danger: true }}
      >
        确定将 <strong>{removeTarget?.name || removeTarget?.username}</strong>{" "}
        从组中移除？
      </Modal>
    </>
  );
}
