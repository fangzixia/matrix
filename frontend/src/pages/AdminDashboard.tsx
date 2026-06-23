import { Link } from "react-router-dom";
import { Card, Typography } from "antd";

export default function AdminDashboard() {
  return (
    <>
      <Typography.Title level={2}>管理概览</Typography.Title>
      <Card>
        <Typography.Paragraph type="secondary">
          欢迎使用 Matrix 管理区域。在此管理用户、查看实例设置并监控平台访问。
        </Typography.Paragraph>
        <Link to="/admin/users">
          <Typography.Link strong>前往用户管理 →</Typography.Link>
        </Link>
      </Card>
    </>
  );
}
