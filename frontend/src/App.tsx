import { RouterProvider } from "react-router-dom";
import { XProvider } from "@ant-design/x";
import zhCN from "antd/locale/zh_CN";
import zhCN_X from "@ant-design/x/locale/zh_CN";
import { router } from "@/router/index";
import { matrixTheme } from "./theme/antd-theme";

export default function App() {
  return (
    <XProvider theme={matrixTheme} locale={{ ...zhCN, ...zhCN_X }}>
      <RouterProvider router={router} />
    </XProvider>
  );
}
