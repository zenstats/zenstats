import { Button } from "@/components/ui/button";
import { useNavigate } from "react-router-dom";

export default function NotFoundPage() {
  const navigate = useNavigate();

  return (
    <div className="flex flex-col items-center justify-center h-screen">
      <h1 className="text-6xl font-bold text-gray-900 mb-4">404</h1>
      <p className="text-2xl text-gray-700 mb-8">页面未找到</p>
      <Button variant="default" onClick={() => navigate("/")}>
        返回首页
      </Button>
    </div>
  );
}
