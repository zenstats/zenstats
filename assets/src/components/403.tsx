import React from "react";

import { Button } from "@/components/ui/button";
import { useNavigate } from "react-router-dom";

export default function Forbidden() {
  const navigate = useNavigate();

  return (
    <div className="flex flex-col items-center justify-center h-screen">
      <h1 className="text-6xl font-bold text-gray-900 mb-4">403 Forbidden</h1>
      <p className="text-2xl text-gray-700 mb-8">无权限访问</p>
      <Button variant="default" onClick={() => navigate("/")}>
        返回首页
      </Button>
    </div>
  );
}
