import { useNavigate } from "react-router-dom";
import { Button, Result } from "antd";

export default function ForbiddenPage() {
  const navigate = useNavigate();
  return (
    <Result
      status="403"
      title="403"
      subTitle="您没有权限访问此页面。"
      extra={
        <Button type="primary" onClick={() => navigate("/projects")}>
          返回项目列表
        </Button>
      }
    />
  );
}
