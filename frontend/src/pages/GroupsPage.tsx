import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Alert,
  Button,
  Empty,
  Flex,
  Input,
  Modal,
  Space,
  Spin,
  Table,
  Typography,
} from "antd";
import { PlusOutlined } from "@ant-design/icons";
import * as groupsApi from "@/api/groups";
import { roleLabels } from "@/api/projects";
import type { Group } from "@/api/groups";

export default function GroupsPage() {
  const navigate = useNavigate();
  const [groups, setGroups] = useState<Group[]>([]);
  const [loading, setLoading] = useState(true);
  const [listError, setListError] = useState("");
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [createError, setCreateError] = useState("");
  const [editOpen, setEditOpen] = useState(false);
  const [editName, setEditName] = useState("");
  const [editTarget, setEditTarget] = useState<Group | null>(null);
  const [editError, setEditError] = useState("");
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Group | null>(null);
  const [deleteError, setDeleteError] = useState("");
  async function load() {
    setLoading(true);
    setListError("");
    try {
      const res = await groupsApi.listGroups();
      setGroups(res.groups);
    } catch (e) {
      setListError(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    load();
  }, []);
  async function create() {
    setCreateError("");
    try {
      const g = await groupsApi.createGroup({ name });
      setOpen(false);
      setName("");
      navigate(`/groups/${g.id}/-/members`);
    } catch (e) {
      setCreateError(e instanceof Error ? e.message : "创建失败");
    }
  }
  function openEdit(g: Group) {
    setEditTarget(g);
    setEditName(g.name);
    setEditError("");
    setEditOpen(true);
  }
  async function saveEdit() {
    if (!editTarget) return;
    setEditError("");
    try {
      await groupsApi.updateGroup(editTarget.id, { name: editName });
      setEditOpen(false);
      await load();
    } catch (e) {
      setEditError(e instanceof Error ? e.message : "保存失败");
    }
  }
  async function confirmDelete() {
    if (!deleteTarget) return;
    setDeleteError("");
    try {
      await groupsApi.deleteGroup(deleteTarget.id);
      setDeleteOpen(false);
      await load();
    } catch (e) {
      setDeleteError(e instanceof Error ? e.message : "删除失败");
    }
  }
  return (
    <div>
      <Flex
        justify="space-between"
        align="flex-start"
        style={{ marginBottom: 16 }}
      >
        <div>
          <Typography.Title level={2} style={{ margin: 0 }}>
            组
          </Typography.Title>
          <Typography.Text
            type="secondary"
            style={{ display: "block", marginTop: 4 }}
          >
            组成员角色会继承到关联项目，与项目直接成员取较高权限。
          </Typography.Text>
        </div>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => setOpen(true)}
        >
          新建组
        </Button>
      </Flex>
      {listError && (
        <Alert type="error" title={listError} style={{ marginBottom: 16 }} />
      )}
      {loading ? (
        <Spin style={{ display: "block", margin: "24px auto" }} />
      ) : !groups.length ? (
        <Empty description="暂无组，点击「新建组」创建。" />
      ) : (
        <Table dataSource={groups} rowKey="id" pagination={false}>
          <Table.Column
            title="名称"
            render={(_, row: Group) => (
              <Link to={`/groups/${row.id}/-/members`}>{row.name}</Link>
            )}
          />
          <Table.Column
            title="我的角色"
            render={(_, row: Group) =>
              row.current_user_role ? roleLabels[row.current_user_role] : "—"
            }
          />
          <Table.Column
            title=""
            render={(_, row: Group) => (
              <Space>
                {row.permissions?.manage_settings && (
                  <Button type="link" onClick={() => openEdit(row)}>
                    重命名
                  </Button>
                )}
                {row.permissions?.delete_group && (
                  <Button
                    type="link"
                    danger
                    onClick={() => {
                      setDeleteTarget(row);
                      setDeleteOpen(true);
                    }}
                  >
                    删除
                  </Button>
                )}
              </Space>
            )}
          />
        </Table>
      )}
      <Modal
        open={open}
        title="新建组"
        onCancel={() => setOpen(false)}
        onOk={create}
        okText="创建"
        okButtonProps={{ disabled: !name.trim() }}
      >
        {createError && (
          <Alert
            type="error"
            title={createError}
            style={{ marginBottom: 12 }}
          />
        )}
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="名称"
        />
      </Modal>
      <Modal
        open={editOpen}
        title="重命名组"
        onCancel={() => setEditOpen(false)}
        onOk={saveEdit}
        okText="保存"
        okButtonProps={{ disabled: !editName.trim() }}
      >
        {editError && (
          <Alert
            type="error"
            title={editError}
            style={{ marginBottom: 12 }}
          />
        )}
        <Input value={editName} onChange={(e) => setEditName(e.target.value)} />
      </Modal>
      <Modal
        open={deleteOpen}
        title="删除组"
        onCancel={() => setDeleteOpen(false)}
        onOk={confirmDelete}
        okText="删除"
        okButtonProps={{ danger: true }}
      >
        {deleteError && (
          <Alert
            type="error"
            title={deleteError}
            style={{ marginBottom: 12 }}
          />
        )}
        确定删除组 <strong>{deleteTarget?.name}</strong>
        ？关联项目的组绑定将被解除，组成员关系将被清除。
      </Modal>
    </div>
  );
}
