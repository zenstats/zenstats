import React from 'react';
import type { RouteObject } from "react-router-dom";
import { Navigate } from "react-router-dom";
import Login from "@/pages/login/login";
import Sites from "@/pages/sites/sites";
import NewSite from "@/pages/sites/new";
import State from "@/pages/sites/stats";
import Setup from '@/pages/login/setup';
import NotFoundPage from '@/pages/404';

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