import { Outlet } from "react-router-dom";
import { Card, Flex, Layout, theme, Typography } from "antd";
import { RobotOutlined } from "@ant-design/icons";
import { Welcome } from "@ant-design/x";
import { MatrixLogo } from "@/components/MatrixLogo";

const { Header, Content, Footer } = Layout;

export function AuthLayout() {
  const { token } = theme.useToken();
  return (
    <Layout
      style={{
        minHeight: "100vh",
        background: `linear-gradient(180deg, ${token.colorBgLayout} 0%, ${token.colorFillAlter} 50%, ${token.colorBgContainer} 100%)`,
      }}
    >
      <Header
        style={{
          display: "flex",
          alignItems: "center",
          padding: "0 24px",
          background: token.colorBgContainer,
          borderBottom: `1px solid ${token.colorBorderSecondary}`,
          boxShadow: token.boxShadow,
        }}
      >
        <MatrixLogo />
      </Header>
      <Content style={{ padding: "48px 24px", flex: 1 }}>
        <Flex
          gap={48}
          align="center"
          justify="center"
          wrap="wrap"
          style={{ maxWidth: 960, margin: "0 auto" }}
        >
          <Flex flex={1} style={{ minWidth: 280, maxWidth: 420 }}>
            <Welcome
              icon={<RobotOutlined />}
              title="完整的 AI 交付平台"
              description="Matrix 将需求、源码、AI 运行与评测整合于单一应用，覆盖从计划到验证的全流程。"
              extra={
                <Typography.Text type="secondary">
                  自托管 · 这是您自托管的 Matrix 实例
                </Typography.Text>
              }
            />
          </Flex>
          <Card
            style={{
              width: 400,
              maxWidth: "100%",
              boxShadow: token.boxShadowSecondary,
              border: `1px solid ${token.colorBorderSecondary}`,
            }}
            styles={{ body: { padding: "32px 28px" } }}
          >
            <Outlet />
          </Card>
        </Flex>
      </Content>
      <Footer style={{ textAlign: "center", background: "transparent" }}>
        <Typography.Text type="secondary">Matrix</Typography.Text>
      </Footer>
    </Layout>
  );
}
