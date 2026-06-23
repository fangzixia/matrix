import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Flex, Spin, Typography } from "antd";
import { useAuthStore } from "@/stores/auth";

export default function SignOutPage() {
  const logout = useAuthStore((s) => s.logout);
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    async function run() {
      try {
        await logout();
      } finally {
        setLoading(false);
        navigate("/users/sign_in", { replace: true });
      }
    }
    run();
  }, [logout, navigate]);
  return (
    <Flex align="center" justify="center" gap={12} style={{ padding: 48 }}>
      {loading && <Spin />}
      <Typography.Text type="secondary">
        {loading ? "退出中…" : "正在跳转…"}
      </Typography.Text>
    </Flex>
  );
}
