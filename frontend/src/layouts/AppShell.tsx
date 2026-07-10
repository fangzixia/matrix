import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Link,
  Outlet,
  useLocation,
  useNavigate,
  useParams,
} from "react-router-dom";
import {
  Alert,
  Avatar,
  Badge,
  Breadcrumb,
  Button,
  Card,
  Dropdown,
  Empty,
  Flex,
  Input,
  Layout,
  List,
  Menu,
  Space,
  theme,
  Typography,
} from "antd";
import type { MenuProps } from "antd";
import {
  PlusOutlined,
  BellOutlined,
  DownOutlined,
  SearchOutlined,
  AppstoreOutlined,
  MessageOutlined,
  FileTextOutlined,
  CodeOutlined,
  CheckCircleOutlined,
  BuildOutlined,
  FolderOutlined,
  SettingOutlined,
} from "@ant-design/icons";
import { useAuthStore } from "@/stores/auth";
import { useProjectStore } from "@/stores/project";
import { useProjectPermissions } from "@/hooks/useProjectPermissions";
import { MatrixLogoLink } from "@/components/MatrixLogo";
import { avatarInitials } from "@/utils/avatar";
import * as notificationsApi from "@/api/notifications";
import type { Notification } from "@/api/notifications";
import { subscribeNotificationStream } from "@/api/stream";
import { formatRelativeTime } from "@/api/projects";

const { Header, Sider, Content } = Layout;

const projectNavIcons = {
  overview: AppstoreOutlined,
  chat: MessageOutlined,
  plan: FileTextOutlined,
  implement: CodeOutlined,
  verify: CheckCircleOutlined,
  build: BuildOutlined,
  repository: FolderOutlined,
  settings: SettingOutlined,
} as const;

type ProjectNavIcon = keyof typeof projectNavIcons;

function useNotificationsMenu(user: unknown, navigate: (to: string) => void) {
  const [notifyOpen, setNotifyOpen] = useState(false);
  const [unreadCount, setUnreadCount] = useState(0);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [notificationError, setNotificationError] = useState("");

  const loadNotifications = useCallback(async () => {
    if (!user) return;
    const [countRes, listRes] = await Promise.all([
      notificationsApi.unreadCount(),
      notificationsApi.listNotifications(),
    ]);
    setNotificationError("");
    setUnreadCount(countRes.count);
    setNotifications(listRes.notifications);
  }, [user]);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        await loadNotifications();
      } catch {
        if (!cancelled) {
          setNotificationError("通知加载失败，将保留最近一次结果。");
        }
      }
    }
    void load();
    const timer = setInterval(load, 30000);
    const unsubscribe = subscribeNotificationStream(() => {
      void load();
    });
    return () => {
      cancelled = true;
      clearInterval(timer);
      unsubscribe();
    };
  }, [loadNotifications]);

  const markAllRead = useCallback(async () => {
    await notificationsApi.markAllRead();
    setUnreadCount(0);
    setNotifications((prev) =>
      prev.map((n) => ({
        ...n,
        read_at: n.read_at || new Date().toISOString(),
      })),
    );
  }, []);

  const markRead = useCallback(
    async (n: Notification) => {
      if (!n.read_at) {
        await notificationsApi.markRead(n.id);
      }
      if (n.link) navigate(n.link);
      setNotifyOpen(false);
      await loadNotifications();
    },
    [loadNotifications, navigate],
  );

  return {
    notifyOpen,
    setNotifyOpen,
    unreadCount,
    notifications,
    notificationError,
    markAllRead,
    markRead,
  };
}

function resolveProjectNavKey(pathname: string, projectId: string): string {
  const base = `/projects/${projectId}`;
  if (pathname === base || pathname === `${base}/`) return "overview";
  if (pathname.includes("/chat")) return "chat";
  if (pathname.includes("/plan")) return "plan";
  if (pathname.includes("/implement")) return "implement";
  if (pathname.includes("/verify")) return "verify";
  if (pathname.includes("/build")) return "build";
  if (pathname.includes("/repository")) return "repository";
  if (pathname.includes("/-/settings")) return "settings";
  return "overview";
}

export function AppShell() {
  const { token } = theme.useToken();
  const user = useAuthStore((s) => s.user);
  const isAdmin = useAuthStore((s) => s.user?.is_admin ?? false);
  const projects = useProjectStore((s) => s.projects);
  const currentProject = useProjectStore((s) => s.current);
  const fetchProjects = useProjectStore((s) => s.fetchProjects);
  const { id: projectId } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const [search, setSearch] = useState("");
  const [projectPickerOpen, setProjectPickerOpen] = useState(false);
  const perms = useProjectPermissions(currentProject);
  const {
    notifyOpen,
    setNotifyOpen,
    unreadCount,
    notifications,
    notificationError,
    markAllRead,
    markRead,
  } = useNotificationsMenu(user, navigate);
  const displayName = user?.name || user?.username;
  const filteredProjects = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return projects;
    return projects.filter((p) => p.name.toLowerCase().includes(q));
  }, [search, projects]);
  const navItems = useMemo(() => {
    if (!projectId)
      return [] as {
        key: string;
        to: string;
        label: string;
        icon: ProjectNavIcon;
      }[];
    const base = `/projects/${projectId}`;
    const items: {
      key: string;
      to: string;
      label: string;
      icon: ProjectNavIcon;
    }[] = [
      { key: "overview", to: base, label: "概览", icon: "overview" },
      { key: "chat", to: `${base}/chat`, label: "对话", icon: "chat" },
      { key: "plan", to: `${base}/plan`, label: "编写计划", icon: "plan" },
      {
        key: "implement",
        to: `${base}/implement`,
        label: "编码实现",
        icon: "implement",
      },
      {
        key: "verify",
        to: `${base}/verify`,
        label: "验证评测",
        icon: "verify",
      },
      { key: "build", to: `${base}/build`, label: "执行构建", icon: "build" },
      {
        key: "repository",
        to: `${base}/repository`,
        label: "仓库",
        icon: "repository",
      },
    ];
    if (perms.canManageSettings) {
      items.push({
        key: "settings",
        to: `${base}/-/settings/general`,
        label: "设置",
        icon: "settings",
      });
    }
    return items;
  }, [projectId, perms.canManageSettings]);
  const siderMenuItems: MenuProps["items"] = useMemo(
    () =>
      navItems.map((item) => {
        const Icon = projectNavIcons[item.icon];
        return { key: item.key, icon: <Icon />, label: item.label };
      }),
    [navItems],
  );
  const selectedNavKey = projectId
    ? resolveProjectNavKey(location.pathname, projectId)
    : "";
  const isFullBleedPage =
    /\/projects\/[^/]+\/(chat|plan)$/.test(location.pathname);
  const headerHeight = token.Layout?.headerHeight ?? 48;
  const userMenuItems: MenuProps["items"] = [
    {
      key: "header",
      type: "group",
      label: (
        <Flex gap={12} align="center">
          <Avatar
            style={{ backgroundColor: token.colorPrimary, flexShrink: 0 }}
          >
            {avatarInitials(displayName)}
          </Avatar>
          <div>
            <Typography.Text strong>{displayName}</Typography.Text>
            <br />
            <Typography.Text type="secondary">
              @{user?.username}
            </Typography.Text>
          </div>
        </Flex>
      ),
    },
    { type: "divider" },
    { key: "profile", label: "编辑资料", onClick: () => navigate("/profile") },
    ...(isAdmin
      ? [{ key: "admin", label: "管理区域", onClick: () => navigate("/admin") }]
      : []),
    { type: "divider" },
    {
      key: "signout",
      label: "退出登录",
      danger: true,
      onClick: () => navigate("/users/sign_out"),
    },
  ];
  useEffect(() => {
    if (!projects.length) {
      fetchProjects();
    }
  }, [projects.length, fetchProjects]);
  useEffect(() => {
    setProjectPickerOpen(false);
    setNotifyOpen(false);
  }, [location.pathname]);
  const projectMenuItems = useMemo((): MenuProps["items"] => {
    const items: MenuProps["items"] = filteredProjects.map((p) => ({
      key: p.id,
      icon: (
        <Avatar
          size={28}
          shape="square"
          style={{ backgroundColor: token.colorPrimary, flexShrink: 0 }}
        >
          {avatarInitials(p.name)}
        </Avatar>
      ),
      label: p.name,
    }));
    if (!items.length) {
      items.push({ key: "empty", label: "未找到项目", disabled: true });
    }
    items.push({ type: "divider" }, { key: "new", label: "创建新项目" });
    return items;
  }, [filteredProjects, token.colorPrimary]);
  function onProjectMenuClick({ key }: { key: string }) {
    setProjectPickerOpen(false);
    if (key === "new") {
      navigate("/projects/new");
    } else if (key !== "empty") {
      navigate(`/projects/${key}`);
    }
  }
  function onProjectPickerOpenChange(open: boolean) {
    setProjectPickerOpen(open);
    if (!open) setSearch("");
  }
  function onSiderMenuClick({ key }: { key: string }) {
    const item = navItems.find((n) => n.key === key);
    if (item) navigate(item.to);
  }
  return (
    <Layout
      style={isFullBleedPage ? { height: "100vh" } : { minHeight: "100vh" }}
    >
      <Header
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "0 16px",
          lineHeight: "normal",
          overflow: "visible",
          background: token.colorBgContainer,
          borderBottom: `1px solid ${token.colorBorderSecondary}`,
          boxShadow: token.boxShadow,
        }}
      >
        <Flex align="center" gap={8} wrap="nowrap">
          <MatrixLogoLink />
          <Dropdown
            open={projectPickerOpen}
            onOpenChange={onProjectPickerOpenChange}
            destroyOnHidden
            placement="bottomLeft"
            trigger={["click"]}
            menu={{
              items: projectMenuItems,
              onClick: onProjectMenuClick,
              style: { maxHeight: 280, overflow: "auto", minWidth: 280 },
            }}
            popupRender={(menu) => (
              <Card size="small" styles={{ body: { padding: 8 } }}>
                <Input
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder="搜索项目"
                  prefix={<SearchOutlined />}
                  allowClear
                  onClick={(e) => e.stopPropagation()}
                  onKeyDown={(e) => e.stopPropagation()}
                  style={{ marginBottom: 4 }}
                />
                {menu}
              </Card>
            )}
          >
            <Button type="text">项目</Button>
          </Dropdown>
          <Link to="/groups">
            <Button type="text">组</Button>
          </Link>
          {projectId && currentProject && (
            <Breadcrumb
              items={[
                {
                  title: (
                    <Link to={`/projects/${projectId}`}>
                      {currentProject.name}
                    </Link>
                  ),
                },
              ]}
              style={{ marginLeft: 8 }}
            />
          )}
        </Flex>
        <Space>
          <Dropdown
            open={notifyOpen}
            onOpenChange={setNotifyOpen}
            destroyOnHidden
            trigger={["click"]}
            placement="bottomRight"
            popupRender={() => (
              <Card
                size="small"
                title="通知"
                extra={
                  unreadCount > 0 ? (
                    <Button type="link" size="small" onClick={markAllRead}>
                      全部已读
                    </Button>
                  ) : undefined
                }
                style={{ width: 320 }}
                styles={{
                  body: { maxHeight: 360, overflow: "auto", padding: 0 },
                }}
              >
                {notificationError && (
                  <Alert
                    type="warning"
                    showIcon
                    title={notificationError}
                    style={{ margin: 8 }}
                  />
                )}
                {!notifications.length ? (
                  <Empty
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                    description="暂无通知"
                  />
                ) : (
                  <List
                    size="small"
                    split
                    dataSource={notifications}
                    renderItem={(n) => (
                      <List.Item
                        onClick={() => markRead(n)}
                        style={{ cursor: "pointer", paddingInline: 16 }}
                      >
                        <List.Item.Meta
                          title={
                            <Typography.Text strong={!n.read_at}>
                              {n.title}
                            </Typography.Text>
                          }
                          description={
                            <>
                              <Typography.Paragraph style={{ marginBottom: 4 }}>
                                {n.body}
                              </Typography.Paragraph>
                              <Typography.Text
                                type="secondary"
                                style={{ fontSize: 12 }}
                              >
                                {formatRelativeTime(n.created_at)}
                              </Typography.Text>
                            </>
                          }
                        />
                      </List.Item>
                    )}
                  />
                )}
              </Card>
            )}
          >
            <Badge count={unreadCount} size="small">
              <Button
                type="text"
                aria-label="通知"
                title="通知"
                icon={<BellOutlined style={{ fontSize: 18 }} />}
              />
            </Badge>
          </Dropdown>
          <Button
            type="text"
            title="新建项目"
            icon={<PlusOutlined />}
            onClick={() => navigate("/projects/new")}
          >
            新建
          </Button>
          <Dropdown
            menu={{ items: userMenuItems }}
            trigger={["click"]}
            placement="bottomRight"
            destroyOnHidden
          >
            <Button type="text">
              <Space size={4}>
                <Avatar
                  size={26}
                  style={{ backgroundColor: token.colorPrimary }}
                >
                  {avatarInitials(displayName)}
                </Avatar>
                <DownOutlined style={{ fontSize: 10 }} />
              </Space>
            </Button>
          </Dropdown>
        </Space>
      </Header>
      <Layout
        style={
          isFullBleedPage
            ? { height: `calc(100vh - ${headerHeight}px)`, minHeight: 0 }
            : undefined
        }
      >
        {projectId && currentProject && (
          <Sider
            width={220}
            theme="light"
            style={{
              background: token.colorFillAlter,
              borderRight: `1px solid ${token.colorBorder}`,
              overflow: "hidden",
            }}
          >
            <Flex
              align="center"
              gap={8}
              style={{
                padding: "16px 16px 12px",
                borderBottom: `1px solid ${token.colorBorderSecondary}`,
              }}
            >
              <Avatar
                shape="square"
                style={{ backgroundColor: token.colorPrimary, flexShrink: 0 }}
              >
                {avatarInitials(currentProject.name)}
              </Avatar>
              <Typography.Text strong ellipsis style={{ flex: 1 }}>
                {currentProject.name}
              </Typography.Text>
            </Flex>
            <Menu
              mode="inline"
              theme="light"
              selectedKeys={[selectedNavKey]}
              items={siderMenuItems}
              onClick={onSiderMenuClick}
              style={{
                borderInlineEnd: "none",
                background: "transparent",
              }}
            />
          </Sider>
        )}
        <Content
          style={
            isFullBleedPage
              ? {
                  padding: 0,
                  flex: 1,
                  height: "100%",
                  minHeight: 0,
                  overflow: "hidden",
                  display: "flex",
                  flexDirection: "column",
                  background: token.colorBgContainer,
                }
              : {
                  padding: 24,
                  background: token.colorBgLayout,
                  minHeight: "100%",
                }
          }
        >
          {isFullBleedPage ? (
            <Flex vertical style={{ height: "100%", flex: 1, minHeight: 0 }}>
              <Outlet />
            </Flex>
          ) : (
            <div style={{ maxWidth: 1280, margin: "0 auto" }}>
              <Outlet />
            </div>
          )}
        </Content>
      </Layout>
    </Layout>
  );
}
