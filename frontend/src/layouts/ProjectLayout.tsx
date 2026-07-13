import { useEffect } from "react";
import { Outlet, useParams } from "react-router-dom";
import { Flex } from "antd";
import { useProjectStore } from "@/stores/project";

export function ProjectLayout() {
  const { id } = useParams();
  const fetchProject = useProjectStore((s) => s.fetchProject);
  useEffect(() => {
    if (id) fetchProject(id);
  }, [id, fetchProject]);
  return (
    <Flex vertical style={{ height: "100%", flex: 1, minHeight: 0 }}>
      <Outlet />
    </Flex>
  );
}
