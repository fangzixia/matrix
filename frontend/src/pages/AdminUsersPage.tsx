import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Avatar,
  Button,
  Flex,
  Input,
  Modal,
  Space,
  Table,
  Tag,
  Typography,
  theme,
} from "antd";
import { PlusOutlined } from "@ant-design/icons";
import * as usersApi from "@/api/users";
import type { UserWithStats } from "@/api/users";
import { blockUser, resetUserPassword, unblockUser } from "@/api/admin";
import { avatarInitials } from "@/utils/avatar";

export default function AdminUsersPage() {
  const { token } = theme.useToken();
  const navigate = useNavigate();
  const [rows, setRows] = useState<UserWithStats[]>([]);
  const [filter, setFilter] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<UserWithStats | null>(null);
  const [resetTarget, setResetTarget] = useState<UserWithStats | null>(null);
  const [newPassword, setNewPassword] = useState("");
  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return rows;
    return rows.filter(
      (u) =>
        u.username.toLowerCase().includes(q) ||
        u.email.toLowerCase().includes(q) ||
        (u.name || "").toLowerCase().includes(q),
    );
  }, [filter, rows]);
  async function load() {
    const res = await usersApi.listUsers();
    setRows(res.users);
  }
  useEffect(() => {
    load();
  }, []);
  async function confirmDelete() {
    if (!deleteTarget) return;
    await usersApi.deleteUser(deleteTarget.id);
    setDeleteTarget(null);
    await load();
  }
  async function confirmReset() {
    if (!resetTarget || !newPassword) return;
    await resetUserPassword(resetTarget.id, newPassword);
    setResetTarget(null);
    setNewPassword("");
  }
  async function toggleBlock(u: UserWithStats) {
    if (u.state === "active") await blockUser(u.id);
    else await unblockUser(u.id);
    await load();
  }
  return (
    <>
      <Flex justify="space-between" align="center" style={{ marginBottom: 16 }}>
        <Typography.Title level={2} style={{ margin: 0 }}>
          用户
        </Typography.Title>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => navigate("/admin/users/new")}
        >
          新建用户
        </Button>
      </Flex>
      <Flex align="center" gap={12} style={{ marginBottom: 16 }}>
        <Input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="搜索用户…"
          style={{ width: 320 }}
        />
        <Typography.Text type="secondary">
          共 {filtered.length} 位用户
        </Typography.Text>
      </Flex>
      <Table dataSource={filtered} rowKey="id" pagination={{ pageSize: 20 }}>
        <Table.Column
          title=""
          width={48}
          render={(_, row: UserWithStats) => (
            <Avatar style={{ backgroundColor: token.colorPrimary }}>
              {avatarInitials(row.name || row.username)}
            </Avatar>
          )}
        />
        <Table.Column title="显示名称" dataIndex="name" />
        <Table.Column title="登录名" dataIndex="username" />
        <Table.Column title="邮箱" dataIndex="email" />
        <Table.Column title="项目数" dataIndex="project_count" />
        <Table.Column
          title="状态"
          render={(_, row: UserWithStats) => (
            <>
              <Tag color={row.state === "active" ? "success" : "error"}>
                {row.state}
              </Tag>
              {row.is_admin && <Tag color="warning">管理员</Tag>}
            </>
          )}
        />
        <Table.Column
          title="上次登录"
          render={(_, row: UserWithStats) =>
            row.last_sign_in_at
              ? new Date(row.last_sign_in_at).toLocaleString()
              : "—"
          }
        />
        <Table.Column
          title="操作"
          render={(_, row: UserWithStats) => (
            <Space>
              <Button
                type="link"
                onClick={() => navigate(`/admin/users/${row.id}`)}
              >
                编辑
              </Button>
              <Button type="link" onClick={() => setResetTarget(row)}>
                重置密码
              </Button>
              <Button type="link" onClick={() => toggleBlock(row)}>
                {row.state === "active" ? "封禁" : "解封"}
              </Button>
              <Button type="link" danger onClick={() => setDeleteTarget(row)}>
                删除
              </Button>
            </Space>
          )}
        />
      </Table>
      <Modal
        open={!!deleteTarget}
        title="删除用户"
        onCancel={() => setDeleteTarget(null)}
        onOk={confirmDelete}
        okText="删除"
        okButtonProps={{ danger: true }}
      >
        确定删除用户 <strong>{deleteTarget?.username}</strong>？
      </Modal>
      <Modal
        open={!!resetTarget}
        title="重置密码"
        onCancel={() => setResetTarget(null)}
        onOk={confirmReset}
        okText="重置"
        okButtonProps={{ disabled: !newPassword }}
      >
        <Input.Password
          value={newPassword}
          onChange={(e) => setNewPassword(e.target.value)}
          placeholder="新密码"
        />
      </Modal>
    </>
  );
}
