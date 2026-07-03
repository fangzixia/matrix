import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import {
  Alert,
  AutoComplete,
  Avatar,
  Button,
  Card,
  Form,
  Modal,
  Select,
  Space,
  Spin,
  Table,
  Typography,
  theme,
} from "antd";
import { useProjectStore } from "@/stores/project";
import { useProjectPermissions } from "@/hooks/useProjectPermissions";
import { useUserSearch } from "@/hooks/useUserSearch";
import * as projectsApi from "@/api/projects";
import type { ProjectMember, MemberRole } from "@/api/projects";
import type { User } from "@/api/auth";
import { avatarInitials } from "@/utils/avatar";
import { pageCardProps } from "@/theme/surface";
import { ProjectSettingsFrame } from "@/pages/project/ProjectSettingsFrame";

export default function SettingsMembersPage() {
  const { token } = theme.useToken();
  const cardProps = pageCardProps(token);
  const { id = "" } = useParams();
  const projectStore = useProjectStore();
  const perms = useProjectPermissions(projectStore.current);
  const [members, setMembers] = useState<ProjectMember[]>([]);
  const [username, setUsername] = useState("");
  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [role, setRole] = useState<MemberRole>("developer");
  const [error, setError] = useState("");
  const [actingUserId, setActingUserId] = useState("");
  const [inviting, setInviting] = useState(false);
  const userSearch = useUserSearch(username, setUsername);
  const roleOptions = projectsApi.memberRoleOptions.map((r) => ({
    value: r.value,
    label: `${r.label} — ${r.hint}`,
    title: r.hint,
  }));
  async function load() {
    const res = await projectsApi.listMembers(id);
    setMembers(res.members);
  }
  useEffect(() => {
    load();
  }, [id]);
  async function invite() {
    setError("");
    const name = username.trim();
    if (!name) return;
    setInviting(true);
    try {
      await projectsApi.addMember(id, { username: name, role });
      setUsername("");
      setSelectedUser(null);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "添加失败");
    } finally {
      setInviting(false);
    }
  }
  async function changeRole(m: ProjectMember, newRole: MemberRole) {
    setError("");
    setActingUserId(m.user_id);
    try {
      await projectsApi.updateMember(id, m.user_id, newRole);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "更新角色失败");
    } finally {
      setActingUserId("");
    }
  }
  async function remove(m: ProjectMember) {
    Modal.confirm({
      title: `移除成员「${m.name || m.username}」？`,
      content: "移除后该用户将失去当前项目权限。",
      okText: "移除",
      okButtonProps: { danger: true },
      cancelText: "取消",
      onOk: async () => {
        setError("");
        setActingUserId(m.user_id);
        try {
          await projectsApi.removeMember(id, m.user_id);
          await load();
        } catch (e) {
          setError(e instanceof Error ? e.message : "移除失败");
        } finally {
          setActingUserId("");
        }
      },
    });
  }
  return (
    <ProjectSettingsFrame
      projectId={id}
      activeTab="members"
      sectionTitle="项目成员"
    >
      {error && (
        <Alert type="error" message={error} showIcon style={{ marginBottom: 16 }} />
      )}
      {!perms.canManageMembers && !error && (
        <Alert
          type="info"
          showIcon
          message="您仅有查看权限。如需邀请或管理成员，请联系项目 Maintainer 或 Owner。"
          style={{ marginBottom: 16 }}
        />
      )}
      {perms.canManageMembers && (
        <Card
          title="邀请成员"
          {...cardProps}
          style={{ ...cardProps.style, marginBottom: 16 }}
        >
          <Space direction="vertical" style={{ width: "100%" }} size="middle">
            <Typography.Text type="secondary">
              按登录名、显示名称或邮箱搜索。
            </Typography.Text>
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
                <strong>{selectedUser.name || selectedUser.username}</strong> (@
                {selectedUser.username})
              </Typography.Text>
            )}
            <Form.Item label="角色" style={{ marginBottom: 0 }}>
              <Select
                value={role}
                onChange={setRole}
                style={{ minWidth: 140 }}
                options={roleOptions}
              />
            </Form.Item>
            <Button
              type="primary"
              loading={inviting}
              disabled={!username.trim()}
              onClick={invite}
            >
              邀请
            </Button>
          </Space>
        </Card>
      )}
      <Card title="成员列表" {...cardProps}>
        <Table dataSource={members} rowKey="user_id" pagination={false}>
        <Table.Column
          title=""
          width={48}
          render={(_, row: ProjectMember) => (
            <Avatar style={{ backgroundColor: token.colorPrimary }}>
              {avatarInitials(row.name || row.username)}
            </Avatar>
          )}
        />
        <Table.Column
          title="账号"
          render={(_, row: ProjectMember) => (
            <>
              <div>{row.name || row.username}</div>
              <Typography.Text type="secondary">
                @{row.username}
              </Typography.Text>
            </>
          )}
        />
        <Table.Column
          title="角色"
          render={(_, row: ProjectMember) =>
            perms.canManageMembers ? (
              <Select
                value={row.role}
                onChange={(r) => changeRole(row, r)}
                loading={actingUserId === row.user_id}
                style={{ minWidth: 140 }}
                options={projectsApi.memberRoleOptions.map((r) => ({
                  value: r.value,
                  label: r.label,
                  title: r.hint,
                }))}
              />
            ) : (
              projectsApi.roleLabels[row.role]
            )
          }
        />
        <Table.Column
          title="加入时间"
          render={(_, row: ProjectMember) =>
            new Date(row.created_at).toLocaleDateString()
          }
        />
        {perms.canManageMembers && (
          <Table.Column
            title="操作"
            render={(_, row: ProjectMember) => (
              <Button
                type="link"
                danger
                loading={actingUserId === row.user_id}
                onClick={() => remove(row)}
              >
                移除
              </Button>
            )}
          />
        )}
      </Table>
      </Card>
    </ProjectSettingsFrame>
  );
}
