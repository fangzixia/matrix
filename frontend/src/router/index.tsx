import { lazy, Suspense, type ComponentType } from "react";
import { createBrowserRouter, Navigate } from "react-router-dom";
import { Flex, Spin } from "antd";
import { AuthGuard } from "./AuthGuard";
import { AppShell } from "@/layouts/AppShell";
import { AuthLayout } from "@/layouts/AuthLayout";
import { AdminLayout } from "@/layouts/AdminLayout";
import { ProjectLayout } from "@/layouts/ProjectLayout";

const lazyPage = (factory: () => Promise<{ default: ComponentType }>) => {
  const Comp = lazy(factory);
  return (
    <Suspense
      fallback={
        <Flex justify="center" style={{ padding: 48 }}>
          <Spin />
        </Flex>
      }
    >
      <Comp />
    </Suspense>
  );
};

export const router = createBrowserRouter([
  {
    element: <AuthLayout />,
    children: [
      {
        element: <AuthGuard public />,
        children: [
          {
            path: "/users/sign_in",
            element: lazyPage(() => import("@/pages/SignInPage")),
          },
          {
            path: "/users/sign_out",
            element: lazyPage(() => import("@/pages/SignOutPage")),
          },
        ],
      },
    ],
  },
  {
    element: <AdminLayout />,
    children: [
      {
        element: <AuthGuard admin />,
        children: [
          {
            path: "/admin",
            element: lazyPage(() => import("@/pages/AdminDashboard")),
          },
          {
            path: "/admin/users",
            element: lazyPage(() => import("@/pages/AdminUsersPage")),
          },
          {
            path: "/admin/users/new",
            element: lazyPage(() => import("@/pages/AdminUserFormPage")),
          },
          {
            path: "/admin/users/:id",
            element: lazyPage(() => import("@/pages/AdminUserFormPage")),
          },
          {
            path: "/admin/system",
            element: <AuthGuard root />,
            children: [
              {
                index: true,
                element: lazyPage(
                  () => import("@/pages/AdminSystemSettingsPage"),
                ),
              },
            ],
          },
        ],
      },
    ],
  },
  {
    element: <AppShell />,
    children: [
      {
        element: <AuthGuard />,
        children: [
          {
            path: "/profile",
            element: lazyPage(() => import("@/pages/ProfilePage")),
          },
          {
            path: "/groups",
            element: lazyPage(() => import("@/pages/GroupsPage")),
          },
          {
            path: "/groups/:id/-/members",
            element: lazyPage(() => import("@/pages/GroupMembersPage")),
          },
          {
            path: "/projects",
            element: lazyPage(() => import("@/pages/ProjectsPage")),
          },
          {
            path: "/projects/new",
            element: lazyPage(() => import("@/pages/ProjectNewPage")),
          },
          {
            path: "/projects/:id",
            element: <ProjectLayout />,
            children: [
              {
                index: true,
                element: lazyPage(() => import("@/pages/project/OverviewPage")),
              },
              {
                path: "chat",
                element: lazyPage(() => import("@/pages/project/ChatPage")),
              },
              {
                path: "plan",
                element: lazyPage(() => import("@/pages/project/StagePage")),
              },
              {
                path: "plan/:taskId",
                element: lazyPage(
                  () => import("@/pages/project/StageTaskDetailPage"),
                ),
              },
              {
                path: "implement",
                element: lazyPage(() => import("@/pages/project/StagePage")),
              },
              {
                path: "implement/:taskId",
                element: lazyPage(
                  () => import("@/pages/project/StageTaskDetailPage"),
                ),
              },
              {
                path: "verify",
                element: lazyPage(() => import("@/pages/project/StagePage")),
              },
              {
                path: "verify/:taskId",
                element: lazyPage(
                  () => import("@/pages/project/StageTaskDetailPage"),
                ),
              },
              {
                path: "build",
                element: lazyPage(() => import("@/pages/project/StagePage")),
              },
              {
                path: "build/:taskId",
                element: lazyPage(
                  () => import("@/pages/project/StageTaskDetailPage"),
                ),
              },
              {
                path: "runs",
                element: <Navigate to="." replace relative="path" />,
              },
              {
                path: "runs/:runId",
                element: lazyPage(
                  () => import("@/pages/project/RunsRedirectPage"),
                ),
              },
              {
                path: "repository",
                element: lazyPage(
                  () => import("@/pages/project/RepositoryPage"),
                ),
              },
              {
                path: "-/settings/general",
                element: lazyPage(
                  () => import("@/pages/project/SettingsGeneralPage"),
                ),
              },
              {
                path: "-/settings/repositories",
                element: lazyPage(
                  () => import("@/pages/project/SettingsRepositoriesPage"),
                ),
              },
              {
                path: "-/settings/members",
                element: lazyPage(
                  () => import("@/pages/project/SettingsMembersPage"),
                ),
              },
            ],
          },
        ],
      },
      {
        element: <AuthGuard public />,
        children: [
          {
            path: "/403",
            element: lazyPage(() => import("@/pages/ForbiddenPage")),
          },
        ],
      },
      { path: "*", element: lazyPage(() => import("@/pages/NotFoundPage")) },
    ],
  },
  { path: "/", element: <Navigate to="/projects" replace /> },
]);
