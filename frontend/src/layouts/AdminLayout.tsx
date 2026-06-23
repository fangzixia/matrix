import { useMemo } from "react";
import { Link, Outlet, useLocation, useNavigate } from "react-router-dom";
import { Button, Layout, Menu, theme, Typography } from "antd";
import type { MenuProps } from "antd";
import {
  DashboardOutlined,
  LeftOutlined,
  SettingOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { MatrixLogo } from "@/components/MatrixLogo";
import { useAuthStore } from "@/stores/auth";

const { Header, Sider, Content } = Layout;

export function AdminLayout() {
  const isRoot = useAuthStore((s) => s.isRoot());
  const location = useLocation();
  const navigate = useNavigate();
  const { token } = theme.useToken();
  const menuItems: MenuProps["items"] = useMemo(() => {
    const items: MenuProps["items"] = [
      { key: "/admin", icon: <DashboardOutlined />, label: "概览" },
      { key: "/admin/users", icon: <UserOutlined />, label: "用户" },
    ];
    if (isRoot) {
      items.push({
        key: "/admin/system",
        icon: <SettingOutlined />,
        label: "系统配置",
      });
    }
    return items;
  }, [isRoot]);
  const selectedKey = useMemo(() => {
    const path = location.pathname;
    if (path.startsWith("/admin/system")) return "/admin/system";
    if (path.startsWith("/admin/users")) return "/admin/users";
    if (path === "/admin" || path === "/admin/") return "/admin";
    return path;
  }, [location.pathname]);
  return (
    <Layout style={{ minHeight: "100vh" }}>
      <Header
        style={{
          display: "flex",
          alignItems: "center",
          gap: 16,
          padding: "0 24px",
          background: token.colorBgContainer,
          borderBottom: `1px solid ${token.colorBorderSecondary}`,
        }}
      >
        <Link to="/projects">
          <Button type="text" icon={<LeftOutlined />}>
            返回应用
          </Button>
        </Link>
        <MatrixLogo showText={false} size={20} />
        <Typography.Title level={5} style={{ margin: 0 }}>
          管理区域
        </Typography.Title>
      </Header>
      <Layout>
        <Sider
          width={220}
          style={{
            background: token.colorBgContainer,
            borderRight: `1px solid ${token.colorBorderSecondary}`,
          }}
        >
          <Menu
            mode="inline"
            selectedKeys={[selectedKey]}
            items={menuItems}
            onClick={({ key }) => navigate(key)}
            style={{ borderInlineEnd: "none", paddingTop: 8 }}
          />
        </Sider>
        <Content style={{ padding: 24, background: token.colorBgLayout }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
