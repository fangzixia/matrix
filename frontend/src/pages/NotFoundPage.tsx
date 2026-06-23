import { useNavigate } from "react-router-dom";
import { Button, Result } from "antd";

export default function NotFoundPage() {
  const navigate = useNavigate();
  return (
    <Result
      status="404"
      title="404"
      subTitle="页面不存在。"
      extra={
        <Button type="primary" onClick={() => navigate("/projects")}>
          返回项目列表
        </Button>
      }
    />
  );
}
