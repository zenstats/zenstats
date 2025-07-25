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
import { SettingsGeneralForm } from './pages/sites/settings/general-form';
import SettingShieldsIpAddress from './pages/sites/settings/shields/ip_address';
import SettingShieldsHostname from './pages/sites/settings/shields/hostname';
import SettingShieldsCountries from './pages/sites/settings/shields/countries';

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

          {
            path: "shields",
            children: [
              {
                path: "ip_address",
                element: <SettingsLayout>
                  <SettingShieldsIpAddress />
                </SettingsLayout>,
              },
              {
                path: "hostname",
                element: <SettingsLayout>
                  <SettingShieldsHostname />
                </SettingsLayout>,
              },
              {
                path: "countries",
                element: <SettingsLayout>
                  <SettingShieldsCountries />
                </SettingsLayout>,
              },
            ]
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