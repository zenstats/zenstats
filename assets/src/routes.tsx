import React from 'react';
import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import Login from "@/pages/login/login";
import Sites from "@/pages/sites/sites";
import NewSite from "@/pages/sites/new";
import State from "@/pages/sites/stats/stats";
import Setup from '@/pages/login/setup';
import NotFoundPage from '@/pages/404';
import SettingsLayout from './pages/sites/settings/layout';
import {SettingsGeneralForm} from './pages/sites/settings/general-form';
const routes: RouteObject[] = [
  {
    path: "/login",
    element: <Login />
  },
  {
    path: "/setup",
    element: <Setup />
  },
  {
    path: "/sites",
    children: [
      {
        index: true,
        element: <Sites />
      },
      {
        path: "new",
        element: <NewSite />
      },
      {
        path: ":domain/stats",
        element: <State />
      },
      {
        path: ":domain/settings",
        children: [
          {
            index: true,
            element: <Navigate to="general" replace />
          },
          {
            path: "general",
            element: <SettingsLayout>
              <SettingsGeneralForm />
            </SettingsLayout>,
          },
        ]
      }
    ]
  },
  {
    path: "/",
    element: <Navigate to="/login" replace />
  },
  {
    path: "*",
    element: <NotFoundPage />
  }
];

export default routes;